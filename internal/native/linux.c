//go:build linux

#include <gtk/gtk.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/XKBlib.h>
#include <X11/extensions/record.h>
#include <X11/extensions/XTest.h>
#include <pthread.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#include "_cgo_export.h"
#include "linux.h"

// ---------------------------------------------------------------------------
// Globals (UI + state live on the GTK main thread; only the record loop runs
// on a worker thread and it merely forwards key events via g_idle_add).
// ---------------------------------------------------------------------------
static GtkWidget *gWindow = NULL;
static GtkWidget *gBox = NULL;       // vertical box holding the rows

static Display *gCtrlDpy = NULL;     // control conn: Xkb, XTest, focus queries
static Display *gDataDpy = NULL;     // data conn: XRecordEnableContext (worker)
static XRecordContext gRC = 0;
static pthread_t gThread;

static int gActive = 0;
static int gShift = 0;
static char gQuery[256];
static int gQueryLen = 0;
static KeySym gTrigger = ':';

static const int kPopupWidth = 340;

// ---------------------------------------------------------------------------
// CSS / window setup
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
    gWindow = gtk_window_new(GTK_WINDOW_POPUP);
    gtk_widget_set_name(gWindow, "jify");
    gtk_window_set_decorated(GTK_WINDOW(gWindow), FALSE);
    gtk_window_set_skip_taskbar_hint(GTK_WINDOW(gWindow), TRUE);
    gtk_window_set_skip_pager_hint(GTK_WINDOW(gWindow), TRUE);
    gtk_window_set_keep_above(GTK_WINDOW(gWindow), TRUE);
    gtk_window_set_accept_focus(GTK_WINDOW(gWindow), FALSE);
    gtk_window_set_type_hint(GTK_WINDOW(gWindow), GDK_WINDOW_TYPE_HINT_TOOLTIP);
    gtk_widget_set_app_paintable(gWindow, TRUE);
    gtk_widget_set_size_request(gWindow, kPopupWidth, -1);

    // Per-pixel transparency for the rounded corners (needs a compositor).
    GdkScreen *screen = gtk_widget_get_screen(gWindow);
    GdkVisual *visual = gdk_screen_get_rgba_visual(screen);
    if (visual) {
        gtk_widget_set_visual(gWindow, visual);
    }

    gBox = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
    gtk_widget_set_name(gBox, "jbox");
    gtk_container_add(GTK_CONTAINER(gWindow), gBox);
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

static void move_near_pointer(void) {
    GdkDisplay *d = gdk_display_get_default();
    GdkSeat *seat = gdk_display_get_default_seat(d);
    GdkDevice *ptr = gdk_seat_get_pointer(seat);
    gint px = 0, py = 0;
    gdk_device_get_position(ptr, NULL, &px, &py);
    gtk_window_move(GTK_WINDOW(gWindow), px, py + 22);
}

static void hide_popup(void) {
    gtk_widget_hide(gWindow);
}

// update_results queries the Go core and rebuilds the popup rows. The first row
// is highlighted because it is what the closing trigger will insert.
static void update_results(void) {
    clear_rows();

    char *res = jifyQuery(gQuery);
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
        gtk_box_pack_start(GTK_BOX(gBox), label, FALSE, FALSE, 0);
        row++;
    }
    free(res);

    if (row == 0) {
        hide_popup();
        return;
    }

    gtk_widget_show_all(gWindow);
    move_near_pointer();
}

// ---------------------------------------------------------------------------
// Text injection via XTest
// ---------------------------------------------------------------------------

static KeyCode find_scratch_keycode(void) {
    int minKc, maxKc;
    XDisplayKeycodes(gCtrlDpy, &minKc, &maxKc);
    int per = 0;
    KeySym *map = XGetKeyboardMapping(gCtrlDpy, minKc, maxKc - minKc + 1, &per);
    KeyCode found = 0;
    for (int kc = minKc; kc <= maxKc && !found; kc++) {
        int empty = 1;
        for (int j = 0; j < per; j++) {
            if (map[(kc - minKc) * per + j] != NoSymbol) { empty = 0; break; }
        }
        if (empty) found = (KeyCode)kc;
    }
    XFree(map);
    return found;
}

static void send_keysym(KeySym ks) {
    KeyCode kc = XKeysymToKeycode(gCtrlDpy, ks);
    int remapped = 0;
    if (kc == 0) {
        kc = find_scratch_keycode();
        if (kc == 0) return;
        KeySym syms[2] = { ks, ks };
        XChangeKeyboardMapping(gCtrlDpy, kc, 2, syms, 1);
        XSync(gCtrlDpy, False);
        remapped = 1;
    }
    XTestFakeKeyEvent(gCtrlDpy, kc, True, 0);
    XTestFakeKeyEvent(gCtrlDpy, kc, False, 0);
    XSync(gCtrlDpy, False);
    if (remapped) {
        KeySym none[2] = { NoSymbol, NoSymbol };
        XChangeKeyboardMapping(gCtrlDpy, kc, 2, none, 1);
        XSync(gCtrlDpy, False);
    }
}

static void send_backspaces(int n) {
    for (int i = 0; i < n; i++) {
        send_keysym(XK_BackSpace);
    }
}

static void send_unicode(const char *utf8) {
    const char *p = utf8;
    while (p && *p) {
        gunichar cp = g_utf8_get_char(p);
        KeySym ks = (cp <= 0xff) ? (KeySym)cp : (KeySym)(cp | 0x01000000);
        send_keysym(ks);
        p = g_utf8_next_char(p);
    }
}

// accept_top replaces the typed ":query:" with the top suggestion's glyph.
static void accept_top(void) {
    char *res = jifyQuery(gQuery);
    if (res == NULL || res[0] == '\0') {
        if (res) free(res);
        gActive = 0;
        hide_popup();
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
    gActive = 0;
    gQueryLen = 0;
    gQuery[0] = '\0';
    hide_popup();

    send_backspaces(toDelete);
    send_unicode(glyph);
}

// ---------------------------------------------------------------------------
// Blacklist (uses the focused window's WM_CLASS)
// ---------------------------------------------------------------------------

static int front_blacklisted(void) {
    Window focus;
    int revert;
    XGetInputFocus(gCtrlDpy, &focus, &revert);
    if (focus == None || focus == PointerRoot) return 0;

    Window w = focus;
    for (int i = 0; i < 8 && w; i++) {
        XClassHint ch;
        if (XGetClassHint(gCtrlDpy, w, &ch)) {
            int bl = 0;
            if (ch.res_class) bl = bl || jifyIsBlacklisted(ch.res_class);
            if (ch.res_name) bl = bl || jifyIsBlacklisted(ch.res_name);
            if (ch.res_name) XFree(ch.res_name);
            if (ch.res_class) XFree(ch.res_class);
            return bl;
        }
        Window root, parent, *children = NULL;
        unsigned int n = 0;
        if (!XQueryTree(gCtrlDpy, w, &root, &parent, &children, &n)) break;
        if (children) XFree(children);
        if (parent == 0 || parent == root) break;
        w = parent;
    }
    return 0;
}

// ---------------------------------------------------------------------------
// Key handling (runs on the GTK main thread via g_idle_add)
// ---------------------------------------------------------------------------

static int is_shortcode(KeySym ks) {
    return (ks >= 'a' && ks <= 'z') || (ks >= 'A' && ks <= 'Z') ||
           (ks >= '0' && ks <= '9') || ks == '_' || ks == '+' || ks == '-';
}

static gboolean handle_key_idle(gpointer data) {
    int packed = GPOINTER_TO_INT(data);
    int press = packed & 1;
    int keycode = packed >> 1;

    KeySym base = XkbKeycodeToKeysym(gCtrlDpy, (KeyCode)keycode, 0, 0);
    if (base == XK_Shift_L || base == XK_Shift_R) {
        gShift = press;
        return G_SOURCE_REMOVE;
    }
    if (!press) return G_SOURCE_REMOVE;

    KeySym ks = XkbKeycodeToKeysym(gCtrlDpy, (KeyCode)keycode, 0, gShift ? 1 : 0);

    if (gActive) {
        if (ks == XK_Escape) {
            gActive = 0;
            gQueryLen = 0;
            gQuery[0] = '\0';
            hide_popup();
        } else if (ks == gTrigger) {
            accept_top();
        } else if (ks == XK_BackSpace) {
            if (gQueryLen > 0) {
                gQuery[--gQueryLen] = '\0';
                update_results();
            } else {
                gActive = 0;
                hide_popup();
            }
        } else if (is_shortcode(ks)) {
            if (gQueryLen < (int)sizeof(gQuery) - 1) {
                gQuery[gQueryLen++] = (char)ks;
                gQuery[gQueryLen] = '\0';
                update_results();
            }
        } else {
            // space, return, or any other key ends the session
            gActive = 0;
            gQueryLen = 0;
            gQuery[0] = '\0';
            hide_popup();
        }
    } else if (ks == gTrigger) {
        if (!front_blacklisted()) {
            gActive = 1;
            gQueryLen = 0;
            gQuery[0] = '\0';
        }
    }
    return G_SOURCE_REMOVE;
}

// ---------------------------------------------------------------------------
// XRecord worker
// ---------------------------------------------------------------------------

static void record_cb(XPointer closure, XRecordInterceptData *data) {
    if (data->category == XRecordFromServer && data->data != NULL) {
        unsigned char *d = (unsigned char *)data->data;
        int type = d[0] & 0x7f;
        int keycode = d[1];
        if (type == KeyPress || type == KeyRelease) {
            int press = (type == KeyPress) ? 1 : 0;
            int packed = (keycode << 1) | press;
            g_idle_add(handle_key_idle, GINT_TO_POINTER(packed));
        }
    }
    XRecordFreeData(data);
}

static void *record_thread(void *arg) {
    (void)arg;
    XRecordEnableContext(gDataDpy, gRC, record_cb, NULL);
    return NULL;
}

static int setup_record(void) {
    int major, minor;
    if (!XRecordQueryVersion(gCtrlDpy, &major, &minor)) {
        fprintf(stderr, "jify: the X server lacks the RECORD extension\n");
        return 0;
    }
    XRecordRange *rr = XRecordAllocRange();
    if (!rr) return 0;
    rr->device_events.first = KeyPress;
    rr->device_events.last = KeyRelease;

    XRecordClientSpec clients = XRecordAllClients;
    gRC = XRecordCreateContext(gCtrlDpy, 0, &clients, 1, &rr, 1);
    XFree(rr);
    if (!gRC) {
        fprintf(stderr, "jify: failed to create XRecord context\n");
        return 0;
    }
    XSync(gCtrlDpy, False);
    return 1;
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

void jifyRun(void) {
    gTrigger = (KeySym)jifyTriggerRune();

    gtk_init(NULL, NULL);

    gCtrlDpy = XOpenDisplay(NULL);
    gDataDpy = XOpenDisplay(NULL);
    if (!gCtrlDpy || !gDataDpy) {
        fprintf(stderr, "jify: cannot open X11 display. The Linux backend "
                        "requires X11 (or XWayland).\n");
        return;
    }

    int xtErr, xtEvt, xtMajor, xtMinor;
    if (!XTestQueryExtension(gCtrlDpy, &xtEvt, &xtErr, &xtMajor, &xtMinor)) {
        fprintf(stderr, "jify: the X server lacks the XTEST extension "
                        "(needed to insert text)\n");
    }

    build_ui();
    apply_css();

    if (!setup_record()) {
        return;
    }
    if (pthread_create(&gThread, NULL, record_thread, NULL) != 0) {
        fprintf(stderr, "jify: failed to start the key capture thread\n");
        return;
    }

    fprintf(stderr, "jify: running. Type \":name:\" to insert an emoji.\n");
    gtk_main();
}
