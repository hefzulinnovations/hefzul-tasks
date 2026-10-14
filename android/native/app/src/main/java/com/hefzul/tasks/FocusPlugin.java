package com.hefzul.tasks;

import android.content.Intent;
import androidx.core.content.ContextCompat;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

/**
 * Shows the ongoing focus notification (see FocusService) and passes its
 * Pause / Resume / Stop buttons back to the web app as "action" events.
 */
@CapacitorPlugin(name = "HefzulFocus")
public class FocusPlugin extends Plugin {

    static final String EXTRA_ACTION = "com.hefzul.tasks.FOCUS_ACTION";

    @Override
    public void load() {
        // The app was started by a notification button.
        handleActionIntent(getActivity().getIntent());
    }

    @Override
    protected void handleOnNewIntent(Intent intent) {
        super.handleOnNewIntent(intent);
        handleActionIntent(intent);
    }

    private void handleActionIntent(Intent intent) {
        if (intent == null) {
            return;
        }
        String action = intent.getStringExtra(EXTRA_ACTION);
        if (action == null) {
            return;
        }
        intent.removeExtra(EXTRA_ACTION);
        JSObject data = new JSObject();
        data.put("action", action);
        // Kept until the web app has subscribed, for a cold start.
        notifyListeners("action", data, true);
    }

    @PluginMethod
    public void show(PluginCall call) {
        Intent intent = new Intent(getContext(), FocusService.class);
        intent.putExtra(FocusService.EXTRA_TITLE, call.getString("title", ""));
        Double endsAt = call.getDouble("endsAt");
        intent.putExtra(FocusService.EXTRA_ENDS_AT, endsAt == null ? 0L : endsAt.longValue());
        intent.putExtra(FocusService.EXTRA_PAUSED, call.getBoolean("paused", false));
        intent.putExtra(FocusService.EXTRA_PAUSED_TEXT, call.getString("pausedText", ""));
        intent.putExtra(FocusService.EXTRA_PAUSE_LABEL, call.getString("pauseLabel", "Pause"));
        intent.putExtra(FocusService.EXTRA_RESUME_LABEL, call.getString("resumeLabel", "Resume"));
        intent.putExtra(FocusService.EXTRA_STOP_LABEL, call.getString("stopLabel", "Stop"));
        try {
            ContextCompat.startForegroundService(getContext(), intent);
            call.resolve();
        } catch (RuntimeException e) {
            // Android 12+ refuses to start a foreground service from the background.
            call.reject("Could not show the focus notification", e);
        }
    }

    @PluginMethod
    public void hide(PluginCall call) {
        getContext().stopService(new Intent(getContext(), FocusService.class));
        call.resolve();
    }
}
