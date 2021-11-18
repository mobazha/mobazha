/**
 * Copyright (c) 2015-present, Facebook, Inc.
 * All rights reserved.
 *
 * This source code is licensed under the BSD-style license found in the
 * LICENSE file in the root directory of this source tree. An additional grant
 * of patent rights can be found in the PATENTS file in the same directory.
 */

#import "AppDelegate.h"
#import <Firebase.h>
#import <AppCenterReactNative.h>
#import <AppCenterReactNativeAnalytics.h>
#import <AppCenterReactNativeCrashes.h>

#import <React/RCTBundleURLProvider.h>
#import <React/RCTRootView.h>
#import <React/RCTLinkingManager.h>

#import "RNBranch.h"

#import <UIKit/UIKit.h>

#import "RNSplashScreen.h"

#import "ServerBridge.h"

@implementation AppDelegate

-(void)initiCloudDocuments {
  NSFileManager* defaultManager = [NSFileManager defaultManager];
  NSURL* iCloudDocumentsURL = [[defaultManager URLForUbiquityContainerIdentifier:NULL] URLByAppendingPathComponent:@"Documents"];
  if ([defaultManager fileExistsAtPath:[iCloudDocumentsURL path] isDirectory:NULL]) {
    [defaultManager createDirectoryAtURL:iCloudDocumentsURL withIntermediateDirectories:true attributes:NULL error:NULL];
  }
}

- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions
{
  [RNBranch initSessionWithLaunchOptions:launchOptions isReferrable:YES];

  NSURL *jsCodeLocation;

  [FIRApp configure];
  [[UNUserNotificationCenter currentNotificationCenter] setDelegate:self];

  [AppCenterReactNative register];  // Initialize AppCenter
  [AppCenterReactNativeAnalytics registerWithInitiallyEnabled:true];  // Initialize AppCenter analytics
  [AppCenterReactNativeCrashes registerWithAutomaticProcessing];  // Initialize AppCenter crashes

  [[NSNotificationCenter defaultCenter] addObserver:self
                                           selector:@selector(deviceWillLockOrBackground:)
                                               name:UIApplicationProtectedDataDidBecomeAvailable
                                             object:nil];

  jsCodeLocation = [[RCTBundleURLProvider sharedSettings] jsBundleURLForBundleRoot:@"index" fallbackResource:nil];

  NSString *deviceName = [[[UIDevice currentDevice] identifierForVendor] UUIDString];
  NSString *serverToken = [NSString stringWithFormat:@"%@%@", @"Token:", deviceName];
  NSDictionary * initialProperties = @{@"server_token" : serverToken};

  RCTRootView *rootView = [[RCTRootView alloc] initWithBundleURL:jsCodeLocation
                                                      moduleName:@"NativeBazaar"
                                               initialProperties: initialProperties
                                                   launchOptions:launchOptions];
  rootView.backgroundColor = [[UIColor alloc] initWithRed:1.0f green:1.0f blue:1.0f alpha:1];

  self.window = [[UIWindow alloc] initWithFrame:[UIScreen mainScreen].bounds];
  UIViewController *rootViewController = [UIViewController new];
  rootViewController.view = rootView;
  self.window.rootViewController = rootViewController;
  [self.window makeKeyAndVisible];

  [ServerBridge startServerThread];

  [RNSplashScreen show];
  
  [self initiCloudDocuments];

  return YES;
}

- (BOOL)application:(UIApplication *)application openURL:(NSURL *)url options:(NSDictionary<UIApplicationOpenURLOptionsKey,id> *)options
{
  if ([RNBranch.branch application:application openURL:url options:options]) {
    return YES;
  }

  if ([RCTLinkingManager application:application openURL:url options:options]) {
    return YES;
  }

  return NO;
}

- (BOOL)application:(UIApplication *)application continueUserActivity:(NSUserActivity *)userActivity restorationHandler:(void (^)(NSArray * _Nullable))restorationHandler
{
  [RCTLinkingManager application:application continueUserActivity:userActivity restorationHandler:restorationHandler];
  return [RNBranch continueUserActivity:userActivity];
}

- (void)deviceWillLockOrBackground:(NSNotification *)notification {
  NSLog(@"GOT HERE");
  [ServerBridge stopServerThread];
  [ServerBridge startServerThread];
}

- (NSURL *)sourceURLForBridge:(RCTBridge *)bridge
{
#if DEBUG
  NSURL *jsUrl = [[RCTBundleURLProvider sharedSettings] jsBundleURLForBundleRoot:@"index" fallbackResource:nil];
  [[RCTBundleURLProvider sharedSettings] setJsLocation:jsUrl.host];
  return jsUrl;
#else
  return [[NSBundle mainBundle] URLForResource:@"main" withExtension:@"jsbundle"];
#endif
}

@end
