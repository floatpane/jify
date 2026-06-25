//go:build windows

package native

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// ---------------------------------------------------------------------------
// Win32 bindings
// ---------------------------------------------------------------------------

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	pSetWindowsHookEx         = user32.NewProc("SetWindowsHookExW")
	pCallNextHookEx           = user32.NewProc("CallNextHookEx")
	pGetMessage               = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessage          = user32.NewProc("DispatchMessageW")
	pDefWindowProc            = user32.NewProc("DefWindowProcW")
	pRegisterClassEx          = user32.NewProc("RegisterClassExW")
	pCreateWindowEx           = user32.NewProc("CreateWindowExW")
	pShowWindow               = user32.NewProc("ShowWindow")
	pSetWindowPos             = user32.NewProc("SetWindowPos")
	pGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pToUnicodeEx              = user32.NewProc("ToUnicodeEx")
	pGetKeyboardState         = user32.NewProc("GetKeyboardState")
	pGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	pGetKeyboardLayout        = user32.NewProc("GetKeyboardLayout")
	pSendInput                = user32.NewProc("SendInput")
	pBeginPaint               = user32.NewProc("BeginPaint")
	pEndPaint                 = user32.NewProc("EndPaint")
	pFillRect                 = user32.NewProc("FillRect")
	pInvalidateRect           = user32.NewProc("InvalidateRect")
	pGetClientRect            = user32.NewProc("GetClientRect")
	pLoadCursor               = user32.NewProc("LoadCursorW")
	pLoadImage                = user32.NewProc("LoadImageW")
	pSendMessage              = user32.NewProc("SendMessageW")
	pDrawText                 = user32.NewProc("DrawTextW")
	pGetCursorPos             = user32.NewProc("GetCursorPos")
	pMonitorFromPoint         = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfo           = user32.NewProc("GetMonitorInfoW")
	pGetGUIThreadInfo         = user32.NewProc("GetGUIThreadInfo")
	pClientToScreen           = user32.NewProc("ClientToScreen")
	pSetWinCompositionAttr    = user32.NewProc("SetWindowCompositionAttribute")

	pCreateFont    = gdi32.NewProc("CreateFontW")
	pSelectObject  = gdi32.NewProc("SelectObject")
	pDeleteObject  = gdi32.NewProc("DeleteObject")
	pSetTextColor  = gdi32.NewProc("SetTextColor")
	pSetBkMode     = gdi32.NewProc("SetBkMode")
	pCreateSolidBr = gdi32.NewProc("CreateSolidBrush")

	pGetModuleHandle      = kernel32.NewProc("GetModuleHandleW")
	pOpenProcess          = kernel32.NewProc("OpenProcess")
	pCloseHandle          = kernel32.NewProc("CloseHandle")
	pQueryFullProcessName = kernel32.NewProc("QueryFullProcessImageNameW")

	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	whKeyboardLL = 13
	hcAction     = 0
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
	wmPaint      = 0x000F
	wmEraseBkgnd = 0x0014
	wmDestroy    = 0x0002

	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008
	wsExNoActivate = 0x08000000

	swHide   = 0
	swShowNA = 8

	swpNoActivate = 0x0010
	swpNoZorder   = 0x0004
	swpShowWindow = 0x0040

	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0D
	vkEscape = 0x1B
	vkUp     = 0x26
	vkDown   = 0x28

	keyEventfKeyUp   = 0x0002
	keyEventfUnicode = 0x0004
	inputKeyboard    = 1

	dtSingleLine  = 0x20
	dtVCenter     = 0x04
	dtLeft        = 0x00
	dtNoPrefix    = 0x800
	dtEndEllipsis = 0x8000

	transparentBk = 1

	dwmaCornerPref = 33
	cornerRound    = 2

	wcaAccentPolicy         = 19
	accentAcrylicBlurBehind = 4

	processQueryLimited = 0x1000

	giCaret = 0x0002

	// Window icon (loaded from the embedded .syso resource, id 1).
	imageIcon     = 1
	lrDefaultSize = 0x0040
	lrShared      = 0x8000
	wmSetIcon     = 0x0080
	iconSmall     = 0
	iconBig       = 1
	appIconResID  = 1

	// Magic marker so our own injected keystrokes are ignored by the hook.
	jifyExtraInfo = 0x4A494659 // "JIFY"

	rowHeight  = 30
	popupWidth = 340
	popupVPad  = 6
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type paintStruct struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     rect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type keybdInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// input mirrors the Win32 INPUT union sized for the keyboard variant on amd64.
type input struct {
	Type uint32
	_    uint32
	Ki   keybdInput
	_    [8]byte
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

type guiThreadInfo struct {
	Size      uint32
	Flags     uint32
	Active    uintptr
	Focus     uintptr
	Capture   uintptr
	MenuOwner uintptr
	MoveSize  uintptr
	Caret     uintptr
	RcCaret   rect
}

type accentPolicy struct {
	AccentState   uint32
	AccentFlags   uint32
	GradientColor uint32
	AnimationId   uint32
}

type winCompositionAttribData struct {
	Attrib uint32
	PvData uintptr
	CbData uintptr
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

type suggestion struct{ glyph, shortcode string }

var (
	hwnd       uintptr
	hookHandle uintptr
	active     bool
	query      []rune
	results    []suggestion
	selected   int
	trigger    rune

	fontText  uintptr
	fontEmoji uintptr
	brushSel  uintptr

	darkTheme bool
)

// Run installs the low-level keyboard hook, creates the popup window and runs
// the Win32 message loop. It blocks until the process exits.
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	trigger = cfg.TriggerRune()
	darkTheme = cfg.Theme != "light"

	instance, _, _ := pGetModuleHandle.Call(0)

	className, _ := syscall.UTF16PtrFromString("JifyPopupClass")
	cursor, _, _ := pLoadCursor.Call(0, 32512 /* IDC_ARROW */)

	// The jify icon is embedded as a PE resource (resource_windows_*.syso),
	// which also gives the .exe its icon in Explorer. Load it for the window.
	icon, _, _ := pLoadImage.Call(instance, appIconResID, imageIcon, 0, 0, lrDefaultSize|lrShared)

	wc := wndClassEx{
		Style:      0,
		WndProc:    syscall.NewCallback(wndProc),
		Instance:   instance,
		Icon:       icon,
		Cursor:     cursor,
		Background: 0, // we paint everything ourselves (acrylic shows through)
		ClassName:  className,
		IconSm:     icon,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if ret, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return err
	}

	title, _ := syscall.UTF16PtrFromString("jify")
	hwnd, _, _ = pCreateWindowEx.Call(
		uintptr(wsExToolWindow|wsExTopmost|wsExNoActivate),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(wsPopup),
		0, 0, popupWidth, rowHeight+2*popupVPad,
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return syscall.GetLastError()
	}

	if icon != 0 {
		pSendMessage.Call(hwnd, wmSetIcon, iconSmall, icon)
		pSendMessage.Call(hwnd, wmSetIcon, iconBig, icon)
	}

	enableAcrylic(hwnd)
	enableRoundedCorners(hwnd)
	createGdiResources()

	hookHandle, _, _ = pSetWindowsHookEx.Call(
		uintptr(whKeyboardLL),
		syscall.NewCallback(hookProc),
		instance,
		0,
	)
	if hookHandle == 0 {
		return syscall.GetLastError()
	}

	// Message loop.
	var m msg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}

func createGdiResources() {
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	emojiFace, _ := syscall.UTF16PtrFromString("Segoe UI Emoji")
	// CreateFontW(height, width, esc, orient, weight, italic, underline, strike,
	//             charset, outPrec, clipPrec, quality, pitch, face)
	fontText, _, _ = pCreateFont.Call(
		^uintptr(17), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4 /*ANTIALIASED*/, 0,
		uintptr(unsafe.Pointer(face)),
	)
	fontEmoji, _, _ = pCreateFont.Call(
		^uintptr(21), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4, 0,
		uintptr(unsafe.Pointer(emojiFace)),
	)
	// Accent colour for the selected row (Windows blue, opaque).
	brushSel, _, _ = pCreateSolidBr.Call(uintptr(rgb(0, 120, 215)))
}

// ---------------------------------------------------------------------------
// Visual effects
// ---------------------------------------------------------------------------

func enableAcrylic(h uintptr) {
	if pSetWinCompositionAttr.Find() != nil {
		return
	}
	var tint uint32 = 0xCC1E1E1E // AABBGGRR: dark, ~80% opaque
	if !darkTheme {
		tint = 0xCCF2F2F2
	}
	policy := accentPolicy{
		AccentState:   accentAcrylicBlurBehind,
		GradientColor: tint,
	}
	data := winCompositionAttribData{
		Attrib: wcaAccentPolicy,
		PvData: uintptr(unsafe.Pointer(&policy)),
		CbData: unsafe.Sizeof(policy),
	}
	pSetWinCompositionAttr.Call(h, uintptr(unsafe.Pointer(&data)))
}

func enableRoundedCorners(h uintptr) {
	if pDwmSetWindowAttribute.Find() != nil {
		return
	}
	pref := int32(cornerRound)
	pDwmSetWindowAttribute.Call(h, uintptr(dwmaCornerPref),
		uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}

func rgb(r, g, b uint32) uint32 { return r | (g << 8) | (b << 16) }

// ---------------------------------------------------------------------------
// Keyboard hook
// ---------------------------------------------------------------------------

func hookProc(nCode uintptr, wParam uintptr, lParam uintptr) uintptr {
	if int32(nCode) != hcAction {
		r, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
		return r
	}
	if wParam != wmKeyDown && wParam != wmSysKeyDown {
		r, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
		return r
	}

	kb := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
	// Ignore the keystrokes we synthesise ourselves.
	if kb.DwExtraInfo == jifyExtraInfo {
		r, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
		return r
	}

	vk := kb.VkCode
	ch := vkToChar(vk, kb.ScanCode)

	if active {
		switch vk {
		case vkEscape:
			hidePopup()
			return 1
		case vkReturn, vkTab:
			if len(results) > 0 {
				acceptSelection()
				return 1
			}
			hidePopup()
		case vkUp:
			if len(results) > 0 {
				selected = (selected - 1 + len(results)) % len(results)
				invalidate()
				return 1
			}
		case vkDown:
			if len(results) > 0 {
				selected = (selected + 1) % len(results)
				invalidate()
				return 1
			}
		case vkBack:
			if len(query) > 0 {
				query = query[:len(query)-1]
				updateResults()
			} else {
				hidePopup()
			}
		default:
			if isShortcodeRune(ch) {
				query = append(query, ch)
				updateResults()
			} else if ch != 0 {
				// Space / punctuation ends the session.
				hidePopup()
			}
		}
	} else if ch == trigger {
		if !frontAppBlacklisted() {
			active = true
			selected = 0
			query = query[:0]
		}
	}

	r, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
	return r
}

// vkToChar resolves the Unicode character produced by a virtual key, honouring
// the active keyboard layout and shift state.
func vkToChar(vk, scan uint32) rune {
	var state [256]byte
	pGetKeyboardState.Call(uintptr(unsafe.Pointer(&state[0])))

	// Inside a low-level hook GetKeyboardState often lags the physical keys, so
	// the modifier bits are patched from the live async state. Without this,
	// Shift-based characters (e.g. ':' on US layouts) are mis-detected.
	patchModifier := func(vkMod int) {
		if r, _, _ := pGetAsyncKeyState.Call(uintptr(vkMod)); r&0x8000 != 0 {
			state[vkMod] |= 0x80
		}
	}
	patchModifier(0x10) // VK_SHIFT
	patchModifier(0x11) // VK_CONTROL
	patchModifier(0x12) // VK_MENU (Alt / AltGr)

	layout, _, _ := pGetKeyboardLayout.Call(0)

	var buf [8]uint16
	// wFlags bit 2 (0x4) => do not change keyboard state (Win 10 1607+).
	r, _, _ := pToUnicodeEx.Call(
		uintptr(vk), uintptr(scan),
		uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buf[0])), 8, 0x4, layout,
	)
	if int32(r) == 1 {
		return rune(buf[0])
	}
	return 0
}

func isShortcodeRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '+' || r == '-'
}

// ---------------------------------------------------------------------------
// Results + popup management
// ---------------------------------------------------------------------------

func updateResults() {
	matches := activeDB.Search(string(query), activeConfig.MaxSuggestions)
	results = results[:0]
	for _, e := range matches {
		results = append(results, suggestion{glyph: e.Char, shortcode: e.Shortcode})
	}
	if len(results) == 0 {
		hideWindowKeepActive()
		return
	}
	if selected >= len(results) {
		selected = 0
	}
	height := int32(len(results)*rowHeight + 2*popupVPad)
	x, y := popupAnchor(height)
	pSetWindowPos.Call(hwnd, ^uintptr(0) /* HWND_TOPMOST */, uintptr(x), uintptr(y),
		popupWidth, uintptr(height), swpNoActivate|swpShowWindow)
	invalidate()
}

// hideWindowKeepActive hides the window but keeps the typing session alive (used
// when the current query has no matches yet).
func hideWindowKeepActive() {
	pShowWindow.Call(hwnd, swHide)
}

func hidePopup() {
	active = false
	query = query[:0]
	results = results[:0]
	selected = 0
	pShowWindow.Call(hwnd, swHide)
}

func invalidate() {
	pInvalidateRect.Call(hwnd, 0, 1)
}

// popupAnchor returns the top-left position for the popup, preferring the text
// caret of the focused control and falling back to the mouse cursor.
func popupAnchor(height int32) (int32, int32) {
	var ax, ay int32
	got := false

	fg, _, _ := pGetForegroundWindow.Call()
	if fg != 0 {
		tid, _, _ := pGetWindowThreadProcessId.Call(fg, 0)
		var gti guiThreadInfo
		gti.Size = uint32(unsafe.Sizeof(gti))
		if r, _, _ := pGetGUIThreadInfo.Call(tid, uintptr(unsafe.Pointer(&gti))); r != 0 && gti.Caret != 0 {
			pt := point{X: gti.RcCaret.Left, Y: gti.RcCaret.Bottom}
			pClientToScreen.Call(gti.Caret, uintptr(unsafe.Pointer(&pt)))
			ax, ay = pt.X, pt.Y+4
			got = true
		}
	}
	if !got {
		var pt point
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		ax, ay = pt.X, pt.Y+20
	}

	// Clamp to the monitor that contains the anchor.
	mon, _, _ := pMonitorFromPoint.Call(uintptr(ax)|uintptr(ay)<<32, 2 /* NEAREST */)
	if mon != 0 {
		var mi monitorInfo
		mi.Size = uint32(unsafe.Sizeof(mi))
		if r, _, _ := pGetMonitorInfo.Call(mon, uintptr(unsafe.Pointer(&mi))); r != 0 {
			if ax+popupWidth > mi.Work.Right {
				ax = mi.Work.Right - popupWidth - 4
			}
			if ax < mi.Work.Left {
				ax = mi.Work.Left + 4
			}
			if ay+height > mi.Work.Bottom {
				ay = ay - height - 24 // flip above the caret
			}
		}
	}
	return ax, ay
}

// ---------------------------------------------------------------------------
// Text replacement
// ---------------------------------------------------------------------------

func acceptSelection() {
	if selected < 0 || selected >= len(results) {
		hidePopup()
		return
	}
	glyph := results[selected].glyph
	toDelete := len(query) + 1 // + trigger character
	hidePopup()

	sendBackspaces(toDelete)
	sendUnicode(glyph)
}

func sendBackspaces(n int) {
	inputs := make([]input, 0, n*2)
	for i := 0; i < n; i++ {
		inputs = append(inputs,
			input{Type: inputKeyboard, Ki: keybdInput{Vk: vkBack, ExtraInfo: jifyExtraInfo}},
			input{Type: inputKeyboard, Ki: keybdInput{Vk: vkBack, Flags: keyEventfKeyUp, ExtraInfo: jifyExtraInfo}},
		)
	}
	sendInputs(inputs)
}

func sendUnicode(s string) {
	units := syscall.StringToUTF16(s) // includes trailing NUL
	inputs := make([]input, 0, len(units)*2)
	for _, u := range units {
		if u == 0 {
			continue
		}
		inputs = append(inputs,
			input{Type: inputKeyboard, Ki: keybdInput{Scan: u, Flags: keyEventfUnicode, ExtraInfo: jifyExtraInfo}},
			input{Type: inputKeyboard, Ki: keybdInput{Scan: u, Flags: keyEventfUnicode | keyEventfKeyUp, ExtraInfo: jifyExtraInfo}},
		)
	}
	sendInputs(inputs)
}

func sendInputs(inputs []input) {
	if len(inputs) == 0 {
		return
	}
	pSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]))
}

// ---------------------------------------------------------------------------
// Blacklist
// ---------------------------------------------------------------------------

func frontAppBlacklisted() bool {
	fg, _, _ := pGetForegroundWindow.Call()
	if fg == 0 {
		return false
	}
	var pid uint32
	pGetWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return false
	}
	h, _, _ := pOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer pCloseHandle.Call(h)

	var buf [260]uint16
	size := uint32(len(buf))
	r, _, _ := pQueryFullProcessName.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return false
	}
	full := syscall.UTF16ToString(buf[:size])
	base := full
	if i := strings.LastIndexAny(full, `\/`); i >= 0 {
		base = full[i+1:]
	}
	return activeConfig.IsBlacklisted(base) || activeConfig.IsBlacklisted(full)
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func wndProc(h, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmEraseBkgnd:
		return 1 // skip default erase so the acrylic backdrop shows through
	case wmPaint:
		paint(h)
		return 0
	case wmDestroy:
		return 0
	}
	r, _, _ := pDefWindowProc.Call(h, message, wParam, lParam)
	return r
}

func paint(h uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(h, uintptr(unsafe.Pointer(&ps)))

	var textColor uint32 = rgb(245, 245, 245)
	var subColor uint32 = rgb(190, 190, 190)
	if !darkTheme {
		textColor = rgb(20, 20, 20)
		subColor = rgb(90, 90, 90)
	}
	pSetBkMode.Call(hdc, transparentBk)

	for i, s := range results {
		top := int32(popupVPad + i*rowHeight)
		rowRect := rect{Left: 4, Top: top, Right: popupWidth - 4, Bottom: top + rowHeight}

		tc := textColor
		sc := subColor
		if i == selected {
			pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rowRect)), brushSel)
			tc = rgb(255, 255, 255)
			sc = rgb(235, 235, 235)
		}

		// Emoji glyph.
		pSelectObject.Call(hdc, fontEmoji)
		pSetTextColor.Call(hdc, uintptr(tc))
		drawText(hdc, s.glyph, rect{Left: 12, Top: top, Right: 48, Bottom: top + rowHeight})

		// Shortcode label.
		pSelectObject.Call(hdc, fontText)
		pSetTextColor.Call(hdc, uintptr(sc))
		drawText(hdc, ":"+s.shortcode+":",
			rect{Left: 50, Top: top, Right: popupWidth - 10, Bottom: top + rowHeight})
	}
}

func drawText(hdc uintptr, text string, r rect) {
	u16, _ := syscall.UTF16PtrFromString(text)
	pDrawText.Call(hdc, uintptr(unsafe.Pointer(u16)), ^uintptr(0),
		uintptr(unsafe.Pointer(&r)),
		dtLeft|dtSingleLine|dtVCenter|dtNoPrefix|dtEndEllipsis)
}
