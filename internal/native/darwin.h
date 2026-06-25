#ifndef JIFY_DARWIN_H
#define JIFY_DARWIN_H

// jifyRun installs the global key event tap, creates the popup panel and runs
// the AppKit main loop. It does not return until the application terminates.
void jifyRun(void);

// jifySetAppIcon stores PNG bytes used as the application icon. Call before
// jifyRun.
void jifySetAppIcon(const void *data, int len);

#endif // JIFY_DARWIN_H
