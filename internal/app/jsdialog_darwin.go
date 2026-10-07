//go:build darwin

package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

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

func ensureJSDialogSupport() {
	C.nexEnsureJSDialogSupport()
}
