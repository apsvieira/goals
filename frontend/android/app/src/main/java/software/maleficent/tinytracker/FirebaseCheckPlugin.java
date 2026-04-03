package software.maleficent.tinytracker;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

import java.lang.reflect.Method;
import java.util.List;

@CapacitorPlugin(name = "FirebaseCheck")
public class FirebaseCheckPlugin extends Plugin {

    @PluginMethod()
    public void isAvailable(PluginCall call) {
        JSObject result = new JSObject();
        try {
            // Use reflection to avoid compile-time dependency on firebase-common
            Class<?> firebaseApp = Class.forName("com.google.firebase.FirebaseApp");
            Method getApps = firebaseApp.getMethod("getApps", android.content.Context.class);
            List<?> apps = (List<?>) getApps.invoke(null, getContext());
            result.put("available", apps != null && !apps.isEmpty());
        } catch (Exception e) {
            result.put("available", false);
        }
        call.resolve(result);
    }
}
