#import "ServerBridge.h"

// import RCTBridge
#if __has_include(<React/RCTBridge.h>)
#import <React/RCTBridge.h>
#elif __has_include(“RCTBridge.h”)
#import “RCTBridge.h”
#else
#import “React/RCTBridge.h” // Required when used as a Pod in a Swift project
#endif

// import RCTEventDispatcher
#if __has_include(<React/RCTEventDispatcher.h>)
#import <React/RCTEventDispatcher.h>
#elif __has_include(“RCTEventDispatcher.h”)
#import “RCTEventDispatcher.h”
#else
#import “React/RCTEventDispatcher.h” // Required when used as a Pod in a Swift project
#endif

#import "ModuleWithEmitter.h"
#import <Mobile/Mobile.h>

MobileNode *obgo;
dispatch_block_t blockTask;

@implementation ServerBridge
@synthesize bridge = _bridge;

// Export a native module
// https://facebook.github.io/react-native/docs/native-modules-ios.html
RCT_EXPORT_MODULE();

// Export constants
// https://facebook.github.io/react-native/releases/next/docs/native-modules-ios.html#exporting-constants
- (NSDictionary *)constantsToExport
{
  return @{
           @"EXAMPLE_CONSTANT": @"example"
         };
}

// Return the native view that represents your React component
- (UIView *)view
{
  return [[UIView alloc] init];
}

// Export methods to a native module
// https://facebook.github.io/react-native/docs/native-modules-ios.html
RCT_EXPORT_METHOD(start)
{
  [ServerBridge startServerThread];
}

RCT_EXPORT_METHOD(stop)
{
  [ServerBridge stopServerThread];
}

#pragma mark - Private methods

// Implement methods that you want to export to the native module
- (void) emitMessageToRN: (NSString *)eventName :(NSDictionary *)params {
  // The bridge eventDispatcher is used to send events from native to JS env
  // No documentation yet on DeviceEventEmitter: https://github.com/facebook/react-native/issues/2819
  [self.bridge.eventDispatcher sendAppEventWithName: eventName body: params];
}

+ (void)startServerThread {
  blockTask = dispatch_block_create(0,^{
    NSError *error;
    NSLog(@"starting up...");
    NSString *deviceName = [[[UIDevice currentDevice] identifierForVendor] UUIDString];
    NSString *serverToken = [NSString stringWithFormat:@"%@%@", @"Token:", deviceName];
    NSArray *paths = NSSearchPathForDirectoriesInDomains(NSDocumentDirectory, NSUserDomainMask, true);
    NSString *documentPath = [paths objectAtIndex:0];

    obgo = MobileNewNode([NSString stringWithFormat:@"%@%@", documentPath, @"/Mobazha"], serverToken, false, @"obmobile", @"", @"", @"", false);
    [obgo start:&error];
    if(error) {
      NSLog(@"%@", error);
      [ModuleWithEmitter emitEventWithName:@"onServerStartFailed" andPayload:nil];
    } else {
      [ModuleWithEmitter emitEventWithName:@"onServerStarted" andPayload:nil];
    }

    while(true) {
      if(dispatch_block_testcancel(blockTask)) {
        [obgo stop:&error];
        if(error) {
          NSLog(@"%@", error);
          [ModuleWithEmitter emitEventWithName:@"onServerStopFailed" andPayload:nil];
        } else {
          blockTask = nil;
          [ModuleWithEmitter emitEventWithName:@"onServerStopped" andPayload:nil];
          break;
        }
      }
      usleep(500000);
    }
  });

  dispatch_time_t time = dispatch_time(DISPATCH_TIME_NOW,2*NSEC_PER_SEC);
  dispatch_after(time,dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), blockTask);

  NSLog(@"STARTING SERVER");
}

+ (void)stopServerThread {
  if (blockTask) {
    dispatch_block_cancel(blockTask);
  } else {
    [ModuleWithEmitter emitEventWithName:@"onServerStopped" andPayload:nil];
  }
}

@end
