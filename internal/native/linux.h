#ifndef JIFY_LINUX_H
#define JIFY_LINUX_H

// jifyRun initialises GTK, opens the X11 record/inject connections, shows the
// popup and runs the GTK main loop. It blocks until the process exits.
void jifyRun(void);

// jifySetIcon stores PNG bytes used as the window/application icon. Call before
// jifyRun.
void jifySetIcon(const void *data, int len);

#endif // JIFY_LINUX_H
