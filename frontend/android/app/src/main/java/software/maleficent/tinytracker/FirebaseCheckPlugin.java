package software.maleficent.tinytracker;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.google.firebase.FirebaseApp;

@CapacitorPlugin(name = "FirebaseCheck")
public class FirebaseCheckPlugin extends Plugin {

    @PluginMethod()
    public void isAvailable(PluginCall call) {
        JSObject result = new JSObject();
        try {
            boolean available = !FirebaseApp.getApps(getContext()).isEmpty();
            result.put("available", available);
        } catch (Exception e) {
            result.put("available", false);
        }
        call.resolve(result);
    }
}
