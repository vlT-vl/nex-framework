//go:build darwin

package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

// webview_go's own WKUIDelegate ("WebviewWKUIDelegate", registered inside the
// dependency at webview creation time) only implements the open-file-panel
// method. It does NOT implement the three WKUIDelegate methods WebKit calls
// for window.alert/confirm/prompt — and WKWebView's contract is that if the
// delegate doesn't implement one of these, the JS call returns immediately
// (confirm() as false, prompt() as null) WITHOUT ever showing anything. This
// silently breaks any app.jsx-style `if (!window.confirm(...)) return` guard
// in front of a privileged API call: every call reads as "cancelled" with no
// dialog ever appearing.
//
// Fixed here by adding the three missing methods to the already-registered
// delegate class at runtime via the Objective-C runtime (class_addMethod is
// valid on a live, already-registered class — method dispatch is resolved
// per-call from the class's method list, not fixed at instantiation time),
// backed by a real modal NSAlert. No changes to the webview_go dependency
// itself are needed or made.
static void nexEnsureJSDialogSupport(void) {
    static BOOL installed = NO;
    if (installed) return;
    installed = YES;

    Class cls = NSClassFromString(@"WebviewWKUIDelegate");
    if (cls == nil) return;

    SEL alertSel = @selector(webView:runJavaScriptAlertPanelWithMessage:initiatedByFrame:completionHandler:);
    if (!class_respondsToSelector(cls, alertSel)) {
        IMP alertImp = imp_implementationWithBlock(^(id self, WKWebView *webView, NSString *message, WKFrameInfo *frame, void (^completionHandler)(void)) {
            NSAlert *alert = [[NSAlert alloc] init];
            alert.messageText = message ?: @"";
            alert.alertStyle = NSAlertStyleInformational;
            [alert addButtonWithTitle:@"OK"];
            [alert runModal];
            completionHandler();
        });
        class_addMethod(cls, alertSel, alertImp, "v@:@@@@");
    }

    SEL confirmSel = @selector(webView:runJavaScriptConfirmPanelWithMessage:initiatedByFrame:completionHandler:);
    if (!class_respondsToSelector(cls, confirmSel)) {
        IMP confirmImp = imp_implementationWithBlock(^(id self, WKWebView *webView, NSString *message, WKFrameInfo *frame, void (^completionHandler)(BOOL)) {
            NSAlert *alert = [[NSAlert alloc] init];
            alert.messageText = message ?: @"";
            alert.alertStyle = NSAlertStyleWarning;
            [alert addButtonWithTitle:@"OK"];
            [alert addButtonWithTitle:@"Cancel"];
            NSModalResponse resp = [alert runModal];
            completionHandler(resp == NSAlertFirstButtonReturn);
        });
        class_addMethod(cls, confirmSel, confirmImp, "v@:@@@@");
    }

    SEL promptSel = @selector(webView:runJavaScriptTextInputPanelWithPrompt:defaultText:initiatedByFrame:completionHandler:);
    if (!class_respondsToSelector(cls, promptSel)) {
        IMP promptImp = imp_implementationWithBlock(^(id self, WKWebView *webView, NSString *prompt, NSString *defaultText, WKFrameInfo *frame, void (^completionHandler)(NSString * _Nullable)) {
            NSAlert *alert = [[NSAlert alloc] init];
            alert.messageText = prompt ?: @"";
            [alert addButtonWithTitle:@"OK"];
            [alert addButtonWithTitle:@"Cancel"];
            NSTextField *input = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 280, 24)];
            input.stringValue = defaultText ?: @"";
            alert.accessoryView = input;
            alert.window.initialFirstResponder = input;
            NSModalResponse resp = [alert runModal];
            completionHandler(resp == NSAlertFirstButtonReturn ? input.stringValue : nil);
        });
        class_addMethod(cls, promptSel, promptImp, "v@:@@@@@");
    }
}
*/
import "C"

// ensureJSDialogSupport patches window.alert/confirm/prompt support into the
// webview's own WKUIDelegate class (macOS only — GTK/WebView2 already show a
// default dialog for these without any extra wiring). Must be called on the
// main thread after webview.New() has created the delegate class (i.e. after
// the webview instance exists), and is idempotent/safe to call more than
// once.
func ensureJSDialogSupport() {
	C.nexEnsureJSDialogSupport()
}
