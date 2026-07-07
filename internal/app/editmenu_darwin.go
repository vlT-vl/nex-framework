//go:build darwin

package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// webview_go never installs an NSMenu, so macOS has nothing to route
// Cmd+C/Cmd+V/Cmd+X/Cmd+A through: copy/paste/cut/select-all silently do
// nothing anywhere in the app (including password/login fields) without a
// minimal Edit menu wired to the standard NSResponder selectors. Installing
// it once on NSApp.mainMenu fixes this framework-wide, for every nex app.
static void nexEnsureEditMenu(void) {
    static BOOL installed = NO;
    if (installed) return;
    installed = YES;

    NSMenu *mainMenu = [NSApp mainMenu];
    if (mainMenu == nil) {
        mainMenu = [[NSMenu alloc] init];
        [NSApp setMainMenu:mainMenu];
    }

    NSMenuItem *editMenuItem = [[NSMenuItem alloc] initWithTitle:@"Edit" action:nil keyEquivalent:@""];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edit"];

    [editMenu addItemWithTitle:@"Undo" action:@selector(undo:) keyEquivalent:@"z"];
    NSMenuItem *redoItem = [editMenu addItemWithTitle:@"Redo" action:@selector(redo:) keyEquivalent:@"z"];
    redoItem.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;
    [editMenu addItem:[NSMenuItem separatorItem]];
    [editMenu addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];
    [editMenu addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
    [editMenu addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];
    [editMenu addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];

    editMenuItem.submenu = editMenu;
    [mainMenu addItem:editMenuItem];
}
*/
import "C"

// ensureEditMenu installs a minimal Edit menu (Undo/Redo/Cut/Copy/Paste/Select
// All) on NSApp.mainMenu so Cmd+C/V/X/A work system-wide. Must be called on
// the main thread after the webview has created NSApplication (i.e. after
// webview.New), and is idempotent/safe to call more than once.
func ensureEditMenu() {
	C.nexEnsureEditMenu()
}
