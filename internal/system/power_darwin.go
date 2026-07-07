//go:build darwin

package system

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <IOKit/ps/IOPowerSources.h>
#include <IOKit/ps/IOPSKeys.h>
#include <CoreFoundation/CoreFoundation.h>

// Reads the same power source data `pmset -g batt` reads, directly via
// IOKit's power-source API — no subprocess spawned.
static int nexBatteryInfo(int *percent, int *charging, int *onACPower, int *hasBattery) {
    *percent = -1;
    *charging = 0;
    *onACPower = 0;
    *hasBattery = 0;

    CFTypeRef info = IOPSCopyPowerSourcesInfo();
    if (info == NULL) return 0;
    CFArrayRef sources = IOPSCopyPowerSourcesList(info);
    if (sources == NULL) {
        CFRelease(info);
        return 0;
    }

    CFIndex count = CFArrayGetCount(sources);
    for (CFIndex i = 0; i < count; i++) {
        CFTypeRef ps = CFArrayGetValueAtIndex(sources, i);
        CFDictionaryRef desc = IOPSGetPowerSourceDescription(info, ps);
        if (desc == NULL) continue;

        CFNumberRef curCap = CFDictionaryGetValue(desc, CFSTR(kIOPSCurrentCapacityKey));
        CFNumberRef maxCap = CFDictionaryGetValue(desc, CFSTR(kIOPSMaxCapacityKey));
        CFBooleanRef isCharging = CFDictionaryGetValue(desc, CFSTR(kIOPSIsChargingKey));
        CFStringRef state = CFDictionaryGetValue(desc, CFSTR(kIOPSPowerSourceStateKey));

        if (curCap != NULL && maxCap != NULL) {
            int cur = 0, max = 0;
            CFNumberGetValue(curCap, kCFNumberIntType, &cur);
            CFNumberGetValue(maxCap, kCFNumberIntType, &max);
            if (max > 0) *percent = (cur * 100) / max;
            *hasBattery = 1;
        }
        if (isCharging != NULL) {
            *charging = CFBooleanGetValue(isCharging) ? 1 : 0;
        }
        if (state != NULL && CFStringCompare(state, CFSTR(kIOPSACPowerValue), 0) == kCFCompareEqualTo) {
            *onACPower = 1;
        }
    }

    CFRelease(sources);
    CFRelease(info);
    return 1;
}
*/
import "C"

// nativePowerInfo replaces `pmset -g batt`.
func nativePowerInfo() map[string]any {
	var percent, charging, onAC, hasBattery C.int
	ok := C.nexBatteryInfo(&percent, &charging, &onAC, &hasBattery)
	out := map[string]any{}
	if ok == 0 {
		return out
	}
	out["onACPower"] = onAC != 0
	out["hasBattery"] = hasBattery != 0
	if hasBattery != 0 {
		out["percent"] = int(percent)
		out["charging"] = charging != 0
	}
	return out
}
