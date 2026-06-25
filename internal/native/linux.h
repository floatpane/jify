#ifndef JIFY_LINUX_H
#define JIFY_LINUX_H

// jifyRun initialises GTK, opens the X11 record/inject connections, shows the
// popup and runs the GTK main loop. It blocks until the process exits.
void jifyRun(void);

#endif // JIFY_LINUX_H
