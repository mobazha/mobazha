package mbz.nativebazaar;

import android.content.Intent;
import android.os.Bundle;
import javax.annotation.Nullable;

import com.facebook.react.ReactActivity;
import com.facebook.react.ReactActivityDelegate;
import io.branch.rnbranch.RNBranchModule;
import org.devio.rn.splashscreen.SplashScreen;
import android.provider.Settings;


public class MainActivity extends ReactActivity {

    /**
     * Returns the name of the main component registered from JavaScript.
     * This is used to schedule rendering of the component.
     */
    @Override
    protected String getMainComponentName() {
        return "NativeBazaar";
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        SplashScreen.show(this);
        super.onCreate(savedInstanceState);
    }

    @Override
    protected void onStart() {
        super.onStart();
        RNBranchModule.initSession(getIntent().getData(), this);
    }

    @Override
    public void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
    }

    @Override
    protected void onResume() {
        super.onResume();
        Intent intent = getIntent();
        setIntent(intent);
        intent.putExtra("branch_force_new_session", true);
    }

    @Override
    protected void onRestart() {
        super.onRestart();
        Intent intent = getIntent();
        setIntent(intent);
        intent.putExtras(this.getIntent());
        intent.putExtra("branch_force_new_session", true);
    }

    @Override
    protected ReactActivityDelegate createReactActivityDelegate() {
        return new ReactActivityDelegate(this, getMainComponentName()) {
            @Nullable
            @Override
            protected Bundle getLaunchOptions() {
                Bundle initialProps = new Bundle();
                String token = ServerConfig.getServerToken();

                initialProps.putString("server_token", token);
                return initialProps;
            }
        };
    }
}
