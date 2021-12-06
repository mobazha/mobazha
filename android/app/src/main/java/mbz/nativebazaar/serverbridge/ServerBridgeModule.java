package mbz.nativebazaar.serverbridge;

import androidx.annotation.NonNull;
import android.os.AsyncTask;
import android.util.Log;
import android.net.Uri;
import android.content.Context;

import java.io.InputStream;
import java.io.OutputStream;
import java.io.FileOutputStream;
import java.io.File;
import java.util.Date;
import java.lang.Runnable;

import com.facebook.react.bridge.ReactApplicationContext;
import com.facebook.react.bridge.ReactContextBaseJavaModule;
import com.facebook.react.bridge.ReactMethod;
import com.facebook.react.bridge.WritableMap;
import com.facebook.react.modules.core.DeviceEventManagerModule;
import com.facebook.react.bridge.ReactContext;
import com.facebook.react.modules.core.DeviceEventManagerModule;

import mbz.nativebazaar.MainApplication;
import mbz.nativebazaar.ServerConfig;
import java.util.HashMap;
import java.util.Map;

import android.net.wifi.WifiManager;
import android.net.wifi.WifiManager.MulticastLock;

import mobile.Mobile;
import mobile.Node;

public class ServerBridgeModule extends ReactContextBaseJavaModule {
    public static final String REACT_CLASS = "ServerBridge";
    private static ReactApplicationContext reactContext = null;
    private static Node serverNode;
    private static ServerTask serverTask;
    private MulticastLock multicastLock = null;

    public ServerBridgeModule(ReactApplicationContext context) {
        // Pass in the context to the constructor and save it so you can emit events
        // https://facebook.github.io/react-native/docs/native-modules-android.html#the-toast-module
        super(context);

        reactContext = context;
    }

    @Override
    public String getName() {
        // Tell React the name of the module
        // https://facebook.github.io/react-native/docs/native-modules-android.html#the-toast-module
        return REACT_CLASS;
    }

    @Override
    public Map<String, Object> getConstants() {
        // Export any constants to be used in your native module
        // https://facebook.github.io/react-native/docs/native-modules-android.html#the-toast-module
        final Map<String, Object> constants = new HashMap<>();
        constants.put("EXAMPLE_CONSTANT", "example");

        return constants;
    }

    @ReactMethod
    public void start () {
        // Acquire multicast lock
        WifiManager wifi = (WifiManager) reactContext.getSystemService(Context.WIFI_SERVICE);
        multicastLock = wifi.createMulticastLock("multicastLock");
        multicastLock.setReferenceCounted(true);
        multicastLock.acquire();

        ServerBridgeModule.startServerThread();
    }

    @ReactMethod
    public void stop () {
        ServerBridgeModule.stopServerThread();

        // Once your finish using it, release multicast lock
        if (multicastLock != null) {
            multicastLock.release();
            multicastLock = null;
        }
    }

    @ReactMethod
    public void downloadFromCloudContentUri(String zipUri) {
        try {
            InputStream input = MainApplication.getAppContext().getContentResolver().openInputStream(Uri.parse(zipUri));
            File file = new File(MainApplication.getAppContext().getCacheDir(), getRandomFileName());
            OutputStream output = new FileOutputStream(file);
            byte[] buffer = new byte[4 * 1024]; // or other buffer size
            int read;

            while ((read = input.read(buffer)) != -1) {
            output.write(buffer, 0, read);
            }
            output.flush();

            final String outputPath = file.getAbsolutePath();
            reactContext
                .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                .emit("onZipFilePathFetched", outputPath);
        } catch (Exception e) {
            reactContext
                .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                .emit("onZipFilePathFetchFailed", null);
        }
    }

    public String getRandomFileName() {
        long millis = System.currentTimeMillis();
        String datetime = new Date().toString();
        datetime = datetime.replace(" ", "");
        datetime = datetime.replace(":", "");
        return "backup_" + datetime + "_" + millis + ".zip";
    }

    public static void startServerThread () {
        serverTask = (ServerTask) new ServerTask().executeOnExecutor(AsyncTask.THREAD_POOL_EXECUTOR);
    }

    public static void stopServerThread () {
        if (serverTask.getStatus() == AsyncTask.Status.RUNNING) {
            serverTask.cancel(true);
        } else {
            reactContext
                .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                .emit("onServerStopped", null);
        }
    }

    private static void emitDeviceEvent(String eventName, @NonNull WritableMap eventData) {
        // A method for emitting from the native side to JS
        // https://facebook.github.io/react-native/docs/native-modules-android.html#sending-events-to-javascript
        reactContext.getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class).emit(eventName, eventData);
    }

    static class ServerTask extends AsyncTask<String, String, String> {

        @Override
        protected void onPreExecute() {
            super.onPreExecute();
            Log.d("ServerBridgeModule", "Started ServerTask...");
        }

        private void stopServer() {
            try {
                Log.d("ServerBridgeModule", "Stopping ServerTask...");
                serverNode.stop();
                Log.d("ServerBridgeModule", "Stopped ServerTask...");
                reactContext
                  .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                  .emit("onServerStopped", null);
            } catch (Exception e) {
                e.printStackTrace();

                reactContext
                  .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                  .emit("onServerStopFailed", null);
            }
        }

        @Override
        protected String doInBackground(String... aurl) {
            final String token = ServerConfig.getServerToken();
            serverNode = Mobile.newNode(
              MainApplication.getAppContext().getFilesDir() + "/Mobazha",
              token,
              false,
              "obmobile",
              "",
              "",
              "",
              false
            );

            try {
                serverNode.start();
                reactContext
                  .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                  .emit("onServerStarted", null);

                while(true) {
                    if(isCancelled()) {
                        stopServer();
                        break;
                    }
                    Thread.sleep(500);
                }
            } catch (Exception e) {
                e.printStackTrace();
                if (e instanceof InterruptedException) {
                    stopServer();
                } else {
                    reactContext
                        .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter.class)
                        .emit("onServerStartFailed", null);
                }
            }
           return null;
        }

        @Override
        protected void onPostExecute(String unused) {
            Log.d("ServerBridgeModule", "Finished ServerTask...");
        }
    }
}
