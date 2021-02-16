package mbz.nativebazaar;

import android.provider.Settings;

public class ServerConfig {
    public static String getServerToken() {
        String token = "Token:";
        String androidId = Settings.Secure.getString(MainApplication.getAppContext().getContentResolver(), Settings.Secure.ANDROID_ID);
        token = token + androidId;
        return token;
    }
}
