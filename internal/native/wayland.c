// Wayland/Hyprland backend for jify on Linux.
// Compiled when CGO_ENABLED=1 and linked into the Linux binary.
//
// Wayland forbids reading other clients' keystrokes (the security model that
// makes XRecord unavailable), so the X11 backend cannot see typing in native
// Wayland apps. This backend instead reads the raw key stream from the kernel
// via libevdev (/dev/input/event*) and translates keycodes to keysyms with
// xkbcommon, honouring the active layout and Shift state. The read is passive
// (we never EVIOCGRAB), so the keys still reach the focused app -- which is
// exactly what the closing-trigger flow needs: the user types ":smile:", then
// we backspace over it and inject the glyph.
//
// Text injection uses `wtype`, which speaks the virtual-keyboard protocol and
// can type arbitrary Unicode into any client (Wayland-native or XWayland).
//
// The popup itself is rendered through XWayland (GDK_BACKEND=x11) because
// Wayland does not let clients position their own top-levels at absolute
// screen coordinates, while gtk_window_move() works fine under XWayland.
//
// Requirements:
//   * the user must be in the 'input' group (read /dev/input/event*)
//   * `wtype` must be installed (emoji insertion)

#define _GNU_SOURCE

#include <gtk/gtk.h>
#include <gtk-layer-shell/gtk-layer-shell.h>
#include <xkbcommon/xkbcommon.h>
#include <libevdev/libevdev.h>
#include <pthread.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <ctype.h>
#include <dirent.h>
#include <poll.h>
#include <sys/ioctl.h>
#include <linux/input.h>

#include "_cgo_export.h"
#include "linux.h"

// ---------------------------------------------------------------------------
// Globals (UI + state machine live on the GTK main thread; only the evdev read
// loop runs on a worker thread, forwarding keysyms via g_idle_add).
// ---------------------------------------------------------------------------
static GtkWidget *gWindow = NULL;
static GtkWidget *gBox = NULL;

static int gActive = 0;
static int gDebug = 0;
static char gQuery[256];
static int gQueryLen = 0;
static xkb_keysym_t gTrigger = ':';

// xkbcommon state (touched only by the evdev worker thread).
static struct xkb_context *gXkbCtx = NULL;
static struct xkb_keymap *gXkbMap = NULL;
static struct xkb_state *gXkbState = NULL;

// Open keyboard devices.
#define MAX_KBD 32
static struct {
    int fd;
    struct libevdev *dev;
} gKbds[MAX_KBD];
static int gKbdCount = 0;
static pthread_t gThread;

extern GdkPixbuf *gIcon; // decoded by jifySetIcon (linux_x11.c)

static const int kPopupWidth = 340;

// jifySetIcon is defined in linux_x11.c and shared by both backends.

// ---------------------------------------------------------------------------
// CSS / window setup (identical to linux_x11.c)
// ---------------------------------------------------------------------------

static void apply_css(void) {
    GtkCssProvider *css = gtk_css_provider_new();
    const char *style =
        "window#jify { background: transparent; }"
        "#jbox {"
        "  background-color: rgba(30,30,30,0.86);"
        "  border-radius: 12px;"
        "  padding: 6px;"
        "  border: 1px solid rgba(255,255,255,0.08);"
        "}"
        ".jrow {"
        "  padding: 5px 12px;"
        "  border-radius: 7px;"
        "  color: #ededed;"
        "  font-size: 14px;"
        "}"
        ".jrow:hover {"
        "  background-color: rgba(255,255,255,0.07);"
        "}"
        ".jrow.sel {"
        "  background-color: rgba(0,120,215,0.95);"
        "  color: #ffffff;"
        "}";
    gtk_css_provider_load_from_data(css, style, -1, NULL);
    gtk_style_context_add_provider_for_screen(
        gdk_screen_get_default(),
        GTK_STYLE_PROVIDER(css),
        GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);
    g_object_unref(css);
}

static void build_ui(void) {
    gWindow = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_widget_set_name(gWindow, "jify");
    gtk_window_set_decorated(GTK_WINDOW(gWindow), FALSE);
    gtk_widget_set_app_paintable(gWindow, TRUE);
    gtk_widget_set_size_request(gWindow, kPopupWidth, -1);

    // Render as a Wayland layer-shell overlay surface: this displays reliably on
    // Hyprland/wlroots (unlike an XWayland override-redirect window) and does not
    // take keyboard focus, so typing still flows to the user's app. Wayland does
    // not expose the text caret or global pointer position, so anchor the popup
    // near the top-centre of the screen instead of chasing the cursor.
    gtk_layer_init_for_window(GTK_WINDOW(gWindow));
    gtk_layer_set_layer(GTK_WINDOW(gWindow), GTK_LAYER_SHELL_LAYER_OVERLAY);
    gtk_layer_set_namespace(GTK_WINDOW(gWindow), "jify");
    gtk_layer_set_keyboard_interactivity(GTK_WINDOW(gWindow), FALSE);
    gtk_layer_set_anchor(GTK_WINDOW(gWindow), GTK_LAYER_SHELL_EDGE_TOP, TRUE);
    gtk_layer_set_margin(GTK_WINDOW(gWindow), GTK_LAYER_SHELL_EDGE_TOP, 120);

    GdkScreen *screen = gtk_widget_get_screen(gWindow);
    GdkVisual *visual = gdk_screen_get_rgba_visual(screen);
    if (visual) {
        gtk_widget_set_visual(gWindow, visual);
    }

    gBox = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
    gtk_widget_set_name(gBox, "jbox");
    gtk_container_add(GTK_CONTAINER(gWindow), gBox);

    if (gIcon) gtk_window_set_icon(GTK_WINDOW(gWindow), gIcon);
}

// ---------------------------------------------------------------------------
// Popup contents
// ---------------------------------------------------------------------------

static void clear_rows(void) {
    GList *children = gtk_container_get_children(GTK_CONTAINER(gBox));
    for (GList *l = children; l != NULL; l = l->next) {
        gtk_widget_destroy(GTK_WIDGET(l->data));
    }
    g_list_free(children);
}

static void hide_popup(void) {
    gtk_widget_hide(gWindow);
}

// end_session tears down an open picker: clears state and hides the popup. Safe
// to call from every exit path.
static void end_session(void) {
    gActive = 0;
    gQueryLen = 0;
    gQuery[0] = '\0';
    hide_popup();
}

// ---------------------------------------------------------------------------
// Text injection via `wtype` (virtual-keyboard protocol)
// ---------------------------------------------------------------------------

static int have_wtype(void) {
    char *p = g_find_program_in_path("wtype");
    if (!p) return 0;
    g_free(p);
    return 1;
}

// replace_with_glyph deletes `backspaces` characters then types `glyph`, all in
// a single wtype invocation so the events stay ordered. wtype processes its
// args left to right: each "-k BackSpace" emits a backspace, the trailing
// positional argument is typed verbatim (emoji are multibyte UTF-8 and never
// start with '-', so they are never mistaken for an option).
static void replace_with_glyph(int backspaces, const char *glyph) {
    if (backspaces < 0) backspaces = 0;
    int n = 1 + backspaces * 2 + (glyph && *glyph ? 1 : 0) + 1;
    char **argv = g_new0(char *, n);
    int i = 0;
    argv[i++] = g_strdup("wtype");
    for (int b = 0; b < backspaces; b++) {
        argv[i++] = g_strdup("-k");
        argv[i++] = g_strdup("BackSpace");
    }
    if (glyph && *glyph) argv[i++] = g_strdup(glyph);
    argv[i] = NULL;

    GError *err = NULL;
    gchar *out = NULL, *errout = NULL;
    gint status = 0;
    if (!g_spawn_sync(NULL, argv, NULL, G_SPAWN_SEARCH_PATH, NULL, NULL,
                      &out, &errout, &status, &err)) {
        fprintf(stderr, "jify: failed to run wtype: %s\n",
                err ? err->message : "unknown error");
        if (err) g_error_free(err);
    } else if (gDebug) {
        fprintf(stderr, "jify: wtype exit=%d%s%s\n", status,
                (errout && *errout) ? " stderr=" : "",
                (errout && *errout) ? errout : "");
    }
    g_free(out);
    g_free(errout);
    g_strfreev(argv);
}

// ---------------------------------------------------------------------------
// Popup rows
// ---------------------------------------------------------------------------

// row_clicked_cb inserts the clicked emoji, replacing the typed ":query" (the
// opening trigger + query; no closing trigger has been typed) with the glyph.
static gboolean row_clicked_cb(GtkWidget *widget, GdkEventButton *event,
                               gpointer user_data) {
    (void)user_data;
    if (event->button != 1) return FALSE;
    const char *g = (const char *)g_object_get_data(G_OBJECT(widget), "jify-glyph");
    if (!g) return TRUE;

    char glyph[64];
    snprintf(glyph, sizeof(glyph), "%s", g);
    int toDelete = gQueryLen + 1; // opening trigger + query (no closing trigger)
    end_session();
    replace_with_glyph(toDelete, glyph);
    return TRUE;
}

static void update_results(void) {
    clear_rows();

    char *res = jifyQuery(gQuery);
    if (gDebug) fprintf(stderr, "jify: query='%s' -> %s\n", gQuery,
                        (res && res[0]) ? "results" : "EMPTY");
    if (res == NULL || res[0] == '\0') {
        if (res) free(res);
        hide_popup();
        return;
    }

    int row = 0;
    char *saveptr = NULL;
    for (char *line = strtok_r(res, "\n", &saveptr);
         line != NULL;
         line = strtok_r(NULL, "\n", &saveptr)) {
        char *tab = strchr(line, '\t');
        if (!tab) continue;
        *tab = '\0';
        const char *glyph = line;
        const char *shortcode = tab + 1;

        GtkWidget *label = gtk_label_new(NULL);
        gtk_widget_set_halign(label, GTK_ALIGN_START);
        gchar *markup = g_markup_printf_escaped(
            "<span size='x-large'>%s</span>  <span foreground='#b9b9b9'>:%s:</span>",
            glyph, shortcode);
        gtk_label_set_markup(GTK_LABEL(label), markup);
        g_free(markup);

        GtkStyleContext *ctx = gtk_widget_get_style_context(label);
        gtk_style_context_add_class(ctx, "jrow");
        if (row == 0) {
            gtk_style_context_add_class(ctx, "sel");
        }

        GtkWidget *ebox = gtk_event_box_new();
        gtk_event_box_set_visible_window(GTK_EVENT_BOX(ebox), FALSE);
        gtk_container_add(GTK_CONTAINER(ebox), label);
        g_object_set_data_full(G_OBJECT(ebox), "jify-glyph", g_strdup(glyph), g_free);
        g_signal_connect(ebox, "button-press-event", G_CALLBACK(row_clicked_cb), NULL);
        gtk_box_pack_start(GTK_BOX(gBox), ebox, FALSE, FALSE, 0);
        row++;
    }
    free(res);

    if (row == 0) {
        hide_popup();
        return;
    }

    gtk_widget_show_all(gWindow);
    if (gDebug)
        fprintf(stderr, "jify: popup shown rows=%d visible=%d\n", row,
                gtk_widget_get_visible(gWindow));
}

// accept_top replaces the typed ":query:" with the top suggestion's glyph. Keys
// are read passively (never grabbed -- grabbing leaves the compositor's modifier
// state stuck), so the whole ":query:" leaked into the app and must be deleted.
static void accept_top(void) {
    char *res = jifyQuery(gQuery);
    if (res == NULL || res[0] == '\0') {
        if (res) free(res);
        end_session();
        return;
    }
    char *nl = strchr(res, '\n');
    if (nl) *nl = '\0';
    char *tab = strchr(res, '\t');
    if (tab) *tab = '\0';
    char glyph[64];
    snprintf(glyph, sizeof(glyph), "%s", res);
    free(res);

    int toDelete = gQueryLen + 2; // opening trigger + query + closing trigger
    if (gDebug)
        fprintf(stderr, "jify: accept glyph='%s' delete=%d\n", glyph, toDelete);
    end_session();
    replace_with_glyph(toDelete, glyph);
}

// ---------------------------------------------------------------------------
// State machine (runs on the GTK main thread via g_idle_add)
// ---------------------------------------------------------------------------

static int is_shortcode(xkb_keysym_t ks) {
    return (ks >= 'a' && ks <= 'z') || (ks >= 'A' && ks <= 'Z') ||
           (ks >= '0' && ks <= '9') || ks == '_' || ks == '+' || ks == '-';
}

// is_modifier reports whether the keysym is a bare modifier (Shift, Ctrl, Alt,
// Super, etc.). These must never end a session -- e.g. the Shift held to type
// the closing ':' (Shift+';') arrives as its own key event before the colon.
static int is_modifier(xkb_keysym_t ks) {
    switch (ks) {
    case XKB_KEY_Shift_L:
    case XKB_KEY_Shift_R:
    case XKB_KEY_Control_L:
    case XKB_KEY_Control_R:
    case XKB_KEY_Alt_L:
    case XKB_KEY_Alt_R:
    case XKB_KEY_Meta_L:
    case XKB_KEY_Meta_R:
    case XKB_KEY_Super_L:
    case XKB_KEY_Super_R:
    case XKB_KEY_Hyper_L:
    case XKB_KEY_Hyper_R:
    case XKB_KEY_Caps_Lock:
    case XKB_KEY_Shift_Lock:
    case XKB_KEY_Num_Lock:
    case XKB_KEY_ISO_Level3_Shift: // AltGr
    case XKB_KEY_ISO_Level5_Shift:
        return 1;
    default:
        return 0;
    }
}

static void process_keysym(xkb_keysym_t ks) {
    if (is_modifier(ks)) return; // never affects the session
    if (gActive) {
        if (ks == XKB_KEY_Escape) {
            end_session();
        } else if (ks == gTrigger) {
            accept_top();
        } else if (ks == XKB_KEY_BackSpace) {
            if (gQueryLen > 0) {
                gQuery[--gQueryLen] = '\0';
                update_results();
            } else {
                end_session();
            }
        } else if (is_shortcode(ks)) {
            if (gQueryLen < (int)sizeof(gQuery) - 1) {
                gQuery[gQueryLen++] = (char)ks;
                gQuery[gQueryLen] = '\0';
                update_results();
            }
        } else {
            // space, return, or any other key ends the session
            end_session();
        }
    } else if (ks == gTrigger) {
        gActive = 1;
        gQueryLen = 0;
        gQuery[0] = '\0';
    }
}

static gboolean handle_key_idle(gpointer data) {
    guint packed = GPOINTER_TO_UINT(data);
    int press = packed & 1;
    xkb_keysym_t ks = (xkb_keysym_t)(packed >> 1);
    if (gDebug) {
        char buf[64];
        xkb_keysym_get_name(ks, buf, sizeof(buf));
        fprintf(stderr, "jify: key %s (0x%x) active=%d\n", buf, ks, gActive);
    }
    if (press) process_keysym(ks);
    return G_SOURCE_REMOVE;
}

// ---------------------------------------------------------------------------
// evdev input capture
// ---------------------------------------------------------------------------

static void init_xkb(void) {
    gXkbCtx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    if (!gXkbCtx) {
        fprintf(stderr, "jify: failed to create xkb context\n");
        return;
    }
    // NULL names => libxkbcommon honours XKB_DEFAULT_LAYOUT/VARIANT/etc. from
    // the environment, falling back to the system default (usually "us").
    struct xkb_rule_names names = {0};
    gXkbMap = xkb_keymap_new_from_names(gXkbCtx, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
    if (!gXkbMap) {
        fprintf(stderr, "jify: failed to compile xkb keymap\n");
        return;
    }
    gXkbState = xkb_state_new(gXkbMap);
}

static int is_keyboard(struct libevdev *dev) {
    return libevdev_has_event_type(dev, EV_KEY) &&
           libevdev_has_event_code(dev, EV_KEY, KEY_A) &&
           libevdev_has_event_code(dev, EV_KEY, KEY_SPACE) &&
           libevdev_has_event_code(dev, EV_KEY, KEY_LEFTSHIFT);
}

static void open_keyboards(void) {
    DIR *d = opendir("/dev/input");
    if (!d) {
        fprintf(stderr, "jify: cannot open /dev/input: %s\n", strerror(errno));
        return;
    }
    struct dirent *e;
    while ((e = readdir(d)) != NULL && gKbdCount < MAX_KBD) {
        if (strncmp(e->d_name, "event", 5) != 0) continue;
        char path[300];
        snprintf(path, sizeof(path), "/dev/input/%s", e->d_name);
        int fd = open(path, O_RDONLY | O_NONBLOCK);
        if (fd < 0) continue;
        struct libevdev *dev = NULL;
        if (libevdev_new_from_fd(fd, &dev) < 0) {
            close(fd);
            continue;
        }
        if (is_keyboard(dev)) {
            gKbds[gKbdCount].fd = fd;
            gKbds[gKbdCount].dev = dev;
            gKbdCount++;
            fprintf(stderr, "jify: monitoring keyboard: %s (%s)\n", path,
                    libevdev_get_name(dev));
        } else {
            libevdev_free(dev);
            close(fd);
        }
    }
    closedir(d);
}

static void *evdev_thread(void *arg) {
    (void)arg;
    struct pollfd pfds[MAX_KBD];
    for (int i = 0; i < gKbdCount; i++) {
        pfds[i].fd = gKbds[i].fd;
        pfds[i].events = POLLIN;
    }

    for (;;) {
        int r = poll(pfds, gKbdCount, -1);
        if (r < 0) {
            if (errno == EINTR) continue;
            break;
        }
        for (int i = 0; i < gKbdCount; i++) {
            if (!(pfds[i].revents & POLLIN)) continue;
            struct input_event ev;
            int rc;
            do {
                rc = libevdev_next_event(gKbds[i].dev, LIBEVDEV_READ_FLAG_NORMAL, &ev);
                if (rc != LIBEVDEV_READ_STATUS_SUCCESS || ev.type != EV_KEY) continue;

                xkb_keycode_t kc = ev.code + 8; // evdev -> xkb keycode offset
                // value: 0 = release, 1 = press, 2 = autorepeat.
                // Resolve the keysym against the *current* modifier state (Shift
                // from an earlier event is already applied) before updating it.
                xkb_keysym_t ks =
                    gXkbState ? xkb_state_key_get_one_sym(gXkbState, kc) : XKB_KEY_NoSymbol;

                if (ev.value == 1 || ev.value == 0) {
                    xkb_state_update_key(gXkbState, kc,
                                         ev.value ? XKB_KEY_DOWN : XKB_KEY_UP);
                }
                if ((ev.value == 1 || ev.value == 2) && ks != XKB_KEY_NoSymbol) {
                    g_idle_add(handle_key_idle,
                               GUINT_TO_POINTER(((guint)ks << 1) | 1u));
                }
            } while (rc == LIBEVDEV_READ_STATUS_SUCCESS ||
                     rc == LIBEVDEV_READ_STATUS_SYNC);
        }
    }
    return NULL;
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

void jifyRunWayland(void) {
    gDebug = (getenv("JIFY_DEBUG") != NULL);
    gTrigger = (xkb_keysym_t)jifyTriggerRune();

    gtk_init(NULL, NULL);
    if (gIcon) gtk_window_set_default_icon(gIcon);

    build_ui();
    apply_css();

    init_xkb();
    open_keyboards();

    if (gKbdCount == 0) {
        fprintf(stderr,
                "jify: no readable keyboards found in /dev/input. Add your user "
                "to the 'input' group and re-login:\n"
                "      sudo usermod -aG input $USER\n");
    }
    if (!have_wtype()) {
        fprintf(stderr,
                "jify: 'wtype' not found in PATH; emoji insertion needs it. "
                "Install it (e.g. 'sudo pacman -S wtype', 'sudo apt install wtype').\n");
    }

    if (pthread_create(&gThread, NULL, evdev_thread, NULL) != 0) {
        fprintf(stderr, "jify: failed to start the key capture thread\n");
        return;
    }

    fprintf(stderr, "jify: running (Wayland/Hyprland backend). "
                    "Type \":name:\" to insert an emoji.\n");
    gtk_main();

    for (int i = 0; i < gKbdCount; i++) {
        if (gKbds[i].dev) libevdev_free(gKbds[i].dev);
        if (gKbds[i].fd >= 0) close(gKbds[i].fd);
    }
}
