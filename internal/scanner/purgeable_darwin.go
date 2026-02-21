//go:build darwin

package scanner

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#include <Foundation/Foundation.h>
#include <stdlib.h>

long long getPurgeableSpace(const char* path) {
    long long purgeable = 0;
    @autoreleasepool {
        NSString *str = [NSString stringWithUTF8String:path];
        NSURL *url = [NSURL fileURLWithPath:str];
        NSError *error = nil;

        NSNumber *importantUsage = nil;
        [url getResourceValue:&importantUsage forKey:NSURLVolumeAvailableCapacityForImportantUsageKey error:&error];

        NSNumber *availableCapacity = nil;
        [url getResourceValue:&availableCapacity forKey:NSURLVolumeAvailableCapacityKey error:&error];

        if (importantUsage != nil && availableCapacity != nil) {
            purgeable = [importantUsage longLongValue] - [availableCapacity longLongValue];
            if (purgeable < 0) {
                purgeable = 0;
            }
        }
    }
    return purgeable;
}
*/
import "C"
import "unsafe"

// GetPurgeableSpace returns the amount of purgeable space in bytes for the volume containing path.
func GetPurgeableSpace(path string) int64 {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	return int64(C.getPurgeableSpace(cPath))
}
