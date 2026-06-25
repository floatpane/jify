//go:build darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#include <ctype.h>
#include "_cgo_export.h"
#include "darwin.h"

// ---------------------------------------------------------------------------
// Layout constants
// ---------------------------------------------------------------------------
static const CGFloat kPanelWidth = 340.0;
static const CGFloat kRowHeight = 30.0;
static const CGFloat kPad = 8.0;
static const CGFloat kCorner = 12.0;

// ---------------------------------------------------------------------------
// Global state (the event tap runs on the main run loop, so no locking needed)
// ---------------------------------------------------------------------------
static CFMachPortRef gTap = NULL;
static NSPanel *gPanel = nil;
static NSView *gBackground = nil;   // NSGlassEffectView or NSVisualEffectView
static NSView *gContainer = nil;    // holds the row views
static NSMutableArray<NSArray *> *gResults = nil; // each: @[glyph, shortcode]
static NSMutableString *gQuery = nil;
static NSInteger gSelected = 0;
static BOOL gActive = NO;
static UniChar gTrigger = ':';
static NSData *gIconData = nil;

// jifySetAppIcon stores the PNG bytes for the application icon (applied in
// jifyRun once NSApplication exists).
void jifySetAppIcon(const void *data, int len) {
    if (data && len > 0) {
        gIconData = [NSData dataWithBytes:data length:(NSUInteger)len];
    }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

static NSTextField *makeLabel(NSRect frame, CGFloat size, BOOL secondary) {
    NSTextField *l = [[NSTextField alloc] initWithFrame:frame];
    l.bezeled = NO;
    l.drawsBackground = NO;
    l.editable = NO;
    l.selectable = NO;
    l.font = [NSFont systemFontOfSize:size];
    l.textColor = secondary ? [NSColor secondaryLabelColor] : [NSColor labelColor];
    l.lineBreakMode = NSLineBreakByTruncatingTail;
    return l;
}

// ensurePanel lazily builds the popup window and its background view.
static void ensurePanel(void) {
    if (gPanel) return;

    NSRect frame = NSMakeRect(0, 0, kPanelWidth, kRowHeight + 2 * kPad);
    gPanel = [[NSPanel alloc]
        initWithContentRect:frame
                  styleMask:(NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel)
                    backing:NSBackingStoreBuffered
                      defer:NO];
    gPanel.opaque = NO;
    gPanel.backgroundColor = [NSColor clearColor];
    gPanel.hasShadow = YES;
    gPanel.level = NSPopUpMenuWindowLevel;
    gPanel.floatingPanel = YES;
    gPanel.becomesKeyOnlyIfNeeded = YES;
    [gPanel setAcceptsMouseMovedEvents:YES];
    gPanel.collectionBehavior =
        NSWindowCollectionBehaviorCanJoinAllSpaces |
        NSWindowCollectionBehaviorFullScreenAuxiliary |
        NSWindowCollectionBehaviorStationary;

    gContainer = [[NSView alloc] initWithFrame:frame];
    gContainer.wantsLayer = YES;

    // Prefer the macOS 26+ "liquid glass" effect; fall back to the frosted
    // NSVisualEffectView on older systems.
    Class glassClass = NSClassFromString(@"NSGlassEffectView");
    if (glassClass) {
        NSView *glass = [[glassClass alloc] initWithFrame:frame];
        @try {
            [glass setValue:@(kCorner) forKey:@"cornerRadius"];
        } @catch (__unused NSException *e) {}
        [glass setValue:gContainer forKey:@"contentView"];
        gBackground = glass;
    } else {
        NSVisualEffectView *vev = [[NSVisualEffectView alloc] initWithFrame:frame];
        vev.material = NSVisualEffectMaterialPopover;
        vev.blendingMode = NSVisualEffectBlendingModeBehindWindow;
        vev.state = NSVisualEffectStateActive;
        vev.wantsLayer = YES;
        vev.layer.cornerRadius = kCorner;
        vev.layer.masksToBounds = YES;
        [vev addSubview:gContainer];
        gBackground = vev;
    }

    gPanel.contentView = gBackground;
}

// axRectToCocoa converts an Accessibility rect (top-left origin on the primary
// screen) to a Cocoa rect (bottom-left origin).
static NSRect axRectToCocoa(CGRect r) {
    CGFloat primaryTop = NSMaxY([[NSScreen screens] firstObject].frame);
    return NSMakeRect(r.origin.x, primaryTop - (r.origin.y + r.size.height),
                      r.size.width, r.size.height);
}

// anchorRectCocoa returns a screen rectangle (Cocoa coordinates) to anchor the
// popup to, preferring the precise text caret and falling back to the focused
// element's frame (i.e. the text field). Returns NO only when the focused app
// exposes neither, in which case the caller uses the mouse location.
static BOOL anchorRectCocoa(NSRect *out) {
    AXUIElementRef sys = AXUIElementCreateSystemWide();
    if (!sys) return NO;

    AXUIElementRef focused = NULL;
    AXError e = AXUIElementCopyAttributeValue(
        sys, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
    CFRelease(sys);
    if (e != kAXErrorSuccess || !focused) return NO;

    BOOL found = NO;

    // 1) Precise caret bounds (best, when the app supports AX text ranges).
    CFTypeRef rangeVal = NULL;
    if (AXUIElementCopyAttributeValue(focused, kAXSelectedTextRangeAttribute,
                                      &rangeVal) == kAXErrorSuccess && rangeVal) {
        CFTypeRef boundsVal = NULL;
        if (AXUIElementCopyParameterizedAttributeValue(
                focused, kAXBoundsForRangeParameterizedAttribute, rangeVal,
                &boundsVal) == kAXErrorSuccess && boundsVal) {
            CGRect r = CGRectZero;
            if (AXValueGetValue((AXValueRef)boundsVal, kAXValueTypeCGRect, &r) &&
                !(r.size.height == 0 && r.origin.x == 0 && r.origin.y == 0)) {
                *out = axRectToCocoa(r);
                found = YES;
            }
            CFRelease(boundsVal);
        }
        CFRelease(rangeVal);
    }

    // 2) Fall back to the focused element's frame, but only when it looks like a
    //    real single-line-ish text field. Terminals and GPUI/Chromium apps
    //    (Ghostty, Zed, Zen, ...) often report a bogus (0,0) origin or a
    //    window-sized element; accepting those would pin the popup to the corner,
    //    so we reject them and let the caller fall back to the mouse.
    if (!found) {
        CFTypeRef posVal = NULL, sizeVal = NULL;
        AXUIElementCopyAttributeValue(focused, kAXPositionAttribute, &posVal);
        AXUIElementCopyAttributeValue(focused, kAXSizeAttribute, &sizeVal);
        if (posVal && sizeVal) {
            CGPoint p = CGPointZero;
            CGSize s = CGSizeZero;
            if (AXValueGetValue((AXValueRef)posVal, kAXValueTypeCGPoint, &p) &&
                AXValueGetValue((AXValueRef)sizeVal, kAXValueTypeCGSize, &s) &&
                s.width > 1 && s.height > 1 && s.height <= 200 &&
                !(p.x == 0 && p.y == 0)) {
                *out = axRectToCocoa(CGRectMake(p.x, p.y, s.width, s.height));
                found = YES;
            }
        }
        if (posVal) CFRelease(posVal);
        if (sizeVal) CFRelease(sizeVal);
    }

    CFRelease(focused);
    return found;
}

// repositionPanel anchors the popup to the focused text field: just below it, or
// directly above when there isn't enough room below. Prefers the precise caret,
// then the field's frame, and only falls back to the mouse when the app exposes
// neither via Accessibility.
static void repositionPanel(CGFloat height) {
    const CGFloat gap = 9.0;

    NSRect anchorRect;
    NSPoint anchor;
    if (anchorRectCocoa(&anchorRect)) {
        anchor = NSMakePoint(NSMidX(anchorRect), anchorRect.origin.y);
    } else {
        NSPoint m = [NSEvent mouseLocation];
        anchorRect = NSMakeRect(m.x, m.y - 8, 0, 16); // pretend a one-line caret
        anchor = m;
    }

    NSScreen *screen = [NSScreen mainScreen];
    for (NSScreen *s in [NSScreen screens]) {
        if (NSPointInRect(anchor, s.frame)) { screen = s; break; }
    }
    NSRect vis = screen.visibleFrame;

    CGFloat anchorBottom = anchorRect.origin.y;
    CGFloat anchorTop = anchorRect.origin.y + anchorRect.size.height;

    CGFloat x = anchorRect.origin.x;
    CGFloat y = anchorBottom - gap - height; // below the field/line

    if (y < NSMinY(vis)) {
        // Not enough room below: sit directly above the field/line instead.
        y = anchorTop + gap;
        if (y + height > NSMaxY(vis)) y = NSMaxY(vis) - height;
    }

    if (x + kPanelWidth > NSMaxX(vis)) x = NSMaxX(vis) - kPanelWidth - 4;
    if (x < NSMinX(vis)) x = NSMinX(vis) + 4;

    [gPanel setFrame:NSMakeRect(x, y, kPanelWidth, height) display:YES];
}

// applyHighlight colours the currently selected row.
static void applyHighlight(void) {
    NSArray<NSView *> *rows = gContainer.subviews;
    for (NSInteger i = 0; i < (NSInteger)rows.count; i++) {
        NSView *row = rows[i];
        row.wantsLayer = YES;
        if (i == gSelected) {
            row.layer.backgroundColor =
                [[NSColor controlAccentColor] colorWithAlphaComponent:0.85].CGColor;
            row.layer.cornerRadius = 6.0;
        } else {
            row.layer.backgroundColor = [NSColor clearColor].CGColor;
        }
    }
}

static void acceptSelection(void);

// JifyRowView is a clickable popup row: clicking inserts that emoji, hovering
// highlights it, and the cursor becomes a pointing hand.
@interface JifyRowView : NSView
@property (nonatomic) NSInteger rowIndex;
@end

@implementation JifyRowView
// Deliver clicks even though the panel never becomes key (non-activating).
- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    (void)event;
    return YES;
}
- (void)mouseUp:(NSEvent *)event {
    (void)event;
    gSelected = self.rowIndex;
    acceptSelection();
}
- (void)mouseEntered:(NSEvent *)event {
    (void)event;
    gSelected = self.rowIndex;
    applyHighlight();
}
- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    for (NSTrackingArea *ta in [self.trackingAreas copy]) {
        [self removeTrackingArea:ta];
    }
    NSTrackingArea *ta = [[NSTrackingArea alloc]
        initWithRect:self.bounds
             options:(NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways |
                      NSTrackingInVisibleRect)
               owner:self
            userInfo:nil];
    [self addTrackingArea:ta];
}
- (void)resetCursorRects {
    [self addCursorRect:self.bounds cursor:[NSCursor pointingHandCursor]];
}
@end

// rebuildRows recreates the row views to match gResults.
static void rebuildRows(void) {
    for (NSView *v in [gContainer.subviews copy]) {
        [v removeFromSuperview];
    }

    NSInteger n = gResults.count;
    CGFloat height = n * kRowHeight + 2 * kPad;

    NSRect bgFrame = NSMakeRect(0, 0, kPanelWidth, height);
    gBackground.frame = bgFrame;
    gContainer.frame = bgFrame;

    for (NSInteger i = 0; i < n; i++) {
        NSArray *entry = gResults[i];
        NSString *glyph = entry[0];
        NSString *shortcode = entry[1];

        // Rows are laid out top-to-bottom (AppKit origin is bottom-left).
        CGFloat y = height - kPad - (i + 1) * kRowHeight;
        NSRect rowFrame = NSMakeRect(kPad / 2, y, kPanelWidth - kPad, kRowHeight);
        JifyRowView *row = [[JifyRowView alloc] initWithFrame:rowFrame];
        row.rowIndex = i;
        row.wantsLayer = YES;

        NSTextField *emojiLabel =
            makeLabel(NSMakeRect(8, 2, 30, kRowHeight - 4), 18, NO);
        emojiLabel.stringValue = glyph;

        NSTextField *nameLabel =
            makeLabel(NSMakeRect(44, 4, kPanelWidth - kPad - 52, kRowHeight - 8), 14, NO);
        nameLabel.stringValue = [NSString stringWithFormat:@":%@:", shortcode];

        [row addSubview:emojiLabel];
        [row addSubview:nameLabel];
        [gContainer addSubview:row];
    }

    repositionPanel(height);
    applyHighlight();
}

// updateResults asks the Go core for matches and refreshes the popup.
static void updateResults(void) {
    char *res = jifyQuery((char *)gQuery.UTF8String);
    NSString *s = res ? [NSString stringWithUTF8String:res] : @"";
    if (res) free(res);

    [gResults removeAllObjects];
    if (s.length > 0) {
        for (NSString *line in [s componentsSeparatedByString:@"\n"]) {
            NSArray *parts = [line componentsSeparatedByString:@"\t"];
            if (parts.count == 2) {
                [gResults addObject:@[parts[0], parts[1]]];
            }
        }
    }

    if (gResults.count == 0) {
        [gPanel orderOut:nil];
        return;
    }

    if (gSelected >= (NSInteger)gResults.count) gSelected = 0;
    rebuildRows();
    [gPanel orderFrontRegardless];
}

static void hidePopup(void) {
    gActive = NO;
    [gQuery setString:@""];
    [gResults removeAllObjects];
    gSelected = 0;
    [gPanel orderOut:nil];
}

// sendBackspaces deletes n characters at the current insertion point.
static void sendBackspaces(int n) {
    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    for (int i = 0; i < n; i++) {
        CGEventRef down = CGEventCreateKeyboardEvent(src, (CGKeyCode)51, true);
        CGEventRef up = CGEventCreateKeyboardEvent(src, (CGKeyCode)51, false);
        CGEventPost(kCGHIDEventTap, down);
        CGEventPost(kCGHIDEventTap, up);
        CFRelease(down);
        CFRelease(up);
    }
    if (src) CFRelease(src);
}

// insertText types a Unicode string at the current insertion point.
static void insertText(NSString *text) {
    NSUInteger len = text.length;
    if (len == 0) return;
    UniChar *buf = malloc(sizeof(UniChar) * len);
    [text getCharacters:buf range:NSMakeRange(0, len)];

    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGEventRef down = CGEventCreateKeyboardEvent(src, 0, true);
    CGEventKeyboardSetUnicodeString(down, len, buf);
    CGEventPost(kCGHIDEventTap, down);
    CGEventRef up = CGEventCreateKeyboardEvent(src, 0, false);
    CGEventKeyboardSetUnicodeString(up, len, buf);
    CGEventPost(kCGHIDEventTap, up);

    CFRelease(down);
    CFRelease(up);
    if (src) CFRelease(src);
    free(buf);
}

// acceptSelection replaces the typed ":query" with the chosen emoji.
static void acceptSelection(void) {
    if (gSelected < 0 || gSelected >= (NSInteger)gResults.count) {
        hidePopup();
        return;
    }
    NSString *glyph = gResults[gSelected][0];
    int toDelete = (int)gQuery.length + 1; // +1 for the trigger character
    hidePopup();

    // Post the edits on the next run-loop tick so they don't interleave with
    // the key event currently being processed by the tap.
    dispatch_async(dispatch_get_main_queue(), ^{
        sendBackspaces(toDelete);
        insertText(glyph);
    });
}

// ---------------------------------------------------------------------------
// Event tap callback
// ---------------------------------------------------------------------------

static CGEventRef tapCallback(CGEventTapProxy proxy, CGEventType type,
                              CGEventRef event, void *ctx) {
    if (type == kCGEventTapDisabledByTimeout ||
        type == kCGEventTapDisabledByUserInput) {
        if (gTap) CGEventTapEnable(gTap, true);
        return event;
    }
    if (type != kCGEventKeyDown) return event;

    CGKeyCode keycode =
        (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);

    UniChar chars[4];
    UniCharCount n = 0;
    CGEventKeyboardGetUnicodeString(event, 4, &n, chars);
    UniChar ch = (n > 0) ? chars[0] : 0;

    if (gActive) {
        switch (keycode) {
            case 53: // escape
                hidePopup();
                return NULL;
            case 36: // return
            case 76: // enter (keypad)
            case 48: // tab
                if (gResults.count > 0) {
                    acceptSelection();
                    return NULL;
                }
                hidePopup();
                return event;
            case 126: // up arrow
                if (gResults.count > 0) {
                    gSelected = (gSelected - 1 + gResults.count) % gResults.count;
                    applyHighlight();
                    return NULL;
                }
                return event;
            case 125: // down arrow
                if (gResults.count > 0) {
                    gSelected = (gSelected + 1) % gResults.count;
                    applyHighlight();
                    return NULL;
                }
                return event;
            case 51: // delete / backspace
                if (gQuery.length > 0) {
                    [gQuery deleteCharactersInRange:NSMakeRange(gQuery.length - 1, 1)];
                    updateResults();
                } else {
                    hidePopup(); // let the backspace delete the trigger char
                }
                return event;
            default:
                if (ch < 128 && (isalnum(ch) || ch == '_' || ch == '+' || ch == '-')) {
                    [gQuery appendFormat:@"%C", ch];
                    updateResults();
                    return event;
                }
                // Space or any other character ends the session.
                hidePopup();
                return event;
        }
    } else if (ch == gTrigger) {
        NSRunningApplication *front =
            [[NSWorkspace sharedWorkspace] frontmostApplication];
        NSString *bid = front.bundleIdentifier;
        if (bid && jifyIsBlacklisted((char *)bid.UTF8String)) {
            return event;
        }
        gActive = YES;
        gSelected = 0;
        [gQuery setString:@""];
        // The popup stays hidden until at least one character is typed.
        return event;
    }

    return event;
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

void jifyRun(void) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];

        if (gIconData) {
            NSImage *icon = [[NSImage alloc] initWithData:gIconData];
            if (icon) [app setApplicationIconImage:icon];
        }

        gQuery = [NSMutableString string];
        gResults = [NSMutableArray array];
        gTrigger = (UniChar)jifyTriggerRune();

        // Prompt for Accessibility permission if not yet granted.
        NSDictionary *opts = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
        BOOL trusted = AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts);

        CGEventMask mask = CGEventMaskBit(kCGEventKeyDown);
        gTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap,
                                kCGEventTapOptionDefault, mask, tapCallback, NULL);
        if (!gTap) {
            NSLog(@"jify: could not create event tap. Grant Accessibility "
                  @"permission in System Settings > Privacy & Security > "
                  @"Accessibility, then relaunch jify. (trusted=%d)", trusted);
        } else {
            CFRunLoopSourceRef rls =
                CFMachPortCreateRunLoopSource(kCFAllocatorDefault, gTap, 0);
            CFRunLoopAddSource(CFRunLoopGetCurrent(), rls, kCFRunLoopCommonModes);
            CGEventTapEnable(gTap, true);
            CFRelease(rls);
            NSLog(@"jify: running. Type the trigger character followed by a name.");
        }

        ensurePanel();
        [app run];
    }
}
