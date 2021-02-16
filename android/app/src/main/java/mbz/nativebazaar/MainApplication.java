package mbz.nativebazaar;

import android.app.Application;

import com.facebook.react.ReactApplication;
import com.rnziparchive.RNZipArchivePackage;

import com.cmcewen.blurview.BlurViewPackage;
import cl.json.ShareApplication;
import cl.json.RNSharePackage;

import com.rnfs.RNFSPackage;
import com.avishayil.rnrestart.ReactNativeRestartPackage;
import com.learnium.RNDeviceInfo.RNDeviceInfo;
import com.ocetnik.timer.BackgroundTimerPackage;
import com.wix.autogrowtextinput.AutoGrowTextInputPackage;
import com.oblador.vectoricons.VectorIconsPackage;
import com.microsoft.appcenter.reactnative.appcenter.AppCenterReactNativePackage;
import com.microsoft.appcenter.reactnative.crashes.AppCenterReactNativeCrashesPackage;
import com.microsoft.appcenter.reactnative.analytics.AppCenterReactNativeAnalyticsPackage;
import com.RNTextInputMask.RNTextInputMaskPackage;
import ui.bottomactionsheet.RNBottomActionSheetPackage;
import com.reactnativecommunity.webview.RNCWebViewPackage;
import com.dylanvann.fastimage.FastImageViewPackage;
import com.horcrux.svg.SvgPackage;
import com.reactnative.ivpusic.imagepicker.PickerPackage;
import com.swmansion.gesturehandler.react.RNGestureHandlerPackage;
import org.reactnative.camera.RNCameraPackage;
import io.branch.rnbranch.RNBranchPackage;
import com.BV.LinearGradient.LinearGradientPackage;
import com.masteratul.exceptionhandler.ReactNativeExceptionHandlerPackage;
import com.facebook.react.ReactNativeHost;
import com.facebook.react.ReactPackage;
import com.facebook.react.shell.MainReactPackage;
import com.facebook.react.PackageList;
import com.facebook.soloader.SoLoader;
import org.devio.rn.splashscreen.SplashScreenReactPackage;
import android.util.Log;
import io.github.elyx0.reactnativedocumentpicker.DocumentPickerPackage;

import io.branch.referral.Branch;

import java.util.Arrays;
import java.util.List;

import android.content.Context;
import android.provider.Settings;

import com.masteratul.exceptionhandler.ReactNativeExceptionHandlerModule;
import com.masteratul.exceptionhandler.NativeExceptionHandlerIfc;

import mbz.nativebazaar.serverbridge.ServerBridgePackage;
import mbz.nativebazaar.serverbridge.ServerBridgeModule;

public class MainApplication extends Application implements ShareApplication, ReactApplication {
  private static Context context;

  @Override
  public String getFileProviderAuthority() {
    return BuildConfig.APPLICATION_ID + ".provider";
  }

  private final ReactNativeHost mReactNativeHost = new ReactNativeHost(this) {
    @Override
    public boolean getUseDeveloperSupport() {
      return BuildConfig.DEBUG;
    }

    @Override
    protected List<ReactPackage> getPackages() {
      @SuppressWarnings("UnnecessaryLocalVariable")
      List<ReactPackage> packages = new PackageList(this).getPackages();
      packages.add(new ServerBridgePackage());
      return packages;
    }

    @Override
    protected String getJSMainModuleName() {
      return "index";
    }
  };

  @Override
  public ReactNativeHost getReactNativeHost() {
    return mReactNativeHost;
  }

  @Override
  public void onCreate() {
    super.onCreate();
    MainApplication.context = getApplicationContext();
    SoLoader.init(this, /* native exopackage */ false);

    ServerBridgeModule.startServerThread();

    Branch.getAutoInstance(this);


    ReactNativeExceptionHandlerModule.setNativeExceptionHandler(new NativeExceptionHandlerIfc() {
      @Override
      public void handleNativeException(Thread thread, Throwable throwable, Thread.UncaughtExceptionHandler originalHandler) {
        // Just empty
      }
    });
  }

  public static Context getAppContext() {
    return MainApplication.context;
  }
}
