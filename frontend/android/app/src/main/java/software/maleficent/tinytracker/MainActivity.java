package software.maleficent.tinytracker;

import android.os.Bundle;
import android.webkit.WebView;

import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsControllerCompat;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        registerPlugin(FirebaseCheckPlugin.class);
        super.onCreate(savedInstanceState);
        WebView.setWebContentsDebuggingEnabled(true);

        // Draw edge-to-edge behind transparent system bars so the web layout's
        // safe-area-inset padding controls content placement, but keep the
        // status bar and Android navigation buttons visible by default.
        WindowCompat.setDecorFitsSystemWindows(getWindow(), false);
        getWindow().setStatusBarColor(android.graphics.Color.TRANSPARENT);
        getWindow().setNavigationBarColor(android.graphics.Color.TRANSPARENT);

        // The app uses a light background, so render the system bar icons dark
        // for contrast.
        WindowInsetsControllerCompat controller =
                WindowCompat.getInsetsController(getWindow(), getWindow().getDecorView());
        controller.setAppearanceLightStatusBars(true);
        controller.setAppearanceLightNavigationBars(true);
    }
}
