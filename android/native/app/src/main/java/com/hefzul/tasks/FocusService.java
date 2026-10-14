package com.hefzul.tasks;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import androidx.core.app.NotificationCompat;
import androidx.core.app.ServiceCompat;

/**
 * Keeps an ongoing notification with the time left while a focus session
 * runs, so Android does not stop the app mid-session. The end-of-session
 * alarm itself is a scheduled local notification from the web app.
 */
public class FocusService extends Service {

    static final String EXTRA_TITLE = "title";
    static final String EXTRA_ENDS_AT = "endsAt";
    static final String EXTRA_PAUSED = "paused";
    static final String EXTRA_PAUSED_TEXT = "pausedText";
    static final String EXTRA_PAUSE_LABEL = "pauseLabel";
    static final String EXTRA_RESUME_LABEL = "resumeLabel";
    static final String EXTRA_STOP_LABEL = "stopLabel";

    private static final String CHANNEL_ID = "focus-timer";
    private static final int NOTIFICATION_ID = 1000;

    private final Handler handler = new Handler(Looper.getMainLooper());
    private final Runnable stopAtEnd = this::stopSelf;

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent == null) {
            stopSelf();
            return START_NOT_STICKY;
        }

        createChannel();
        boolean paused = intent.getBooleanExtra(EXTRA_PAUSED, false);
        long endsAt = intent.getLongExtra(EXTRA_ENDS_AT, 0L);

        int type = Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE
            ? ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE
            : 0;
        ServiceCompat.startForeground(this, NOTIFICATION_ID, buildNotification(intent, paused, endsAt), type);

        handler.removeCallbacks(stopAtEnd);
        if (!paused) {
            long delay = endsAt - System.currentTimeMillis();
            if (endsAt <= 0 || delay <= 0) {
                stopSelf();
                return START_NOT_STICKY;
            }
            handler.postDelayed(stopAtEnd, delay);
        }
        return START_NOT_STICKY;
    }

    @Override
    public void onDestroy() {
        handler.removeCallbacks(stopAtEnd);
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE);
        super.onDestroy();
    }

    private void createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return;
        }
        NotificationChannel channel = new NotificationChannel(
            CHANNEL_ID,
            getString(R.string.focus_channel_name),
            NotificationManager.IMPORTANCE_LOW
        );
        channel.setShowBadge(false);
        getSystemService(NotificationManager.class).createNotificationChannel(channel);
    }

    private Notification buildNotification(Intent intent, boolean paused, long endsAt) {
        NotificationCompat.Builder builder = new NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_hefzul)
            .setContentTitle(intent.getStringExtra(EXTRA_TITLE))
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setCategory(NotificationCompat.CATEGORY_PROGRESS)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .setContentIntent(openApp(null, 0));

        if (paused) {
            builder.setContentText(intent.getStringExtra(EXTRA_PAUSED_TEXT))
                .setShowWhen(false)
                .addAction(0, intent.getStringExtra(EXTRA_RESUME_LABEL), openApp("resume", 1));
        } else {
            builder.setUsesChronometer(true)
                .setChronometerCountDown(true)
                .setWhen(endsAt)
                .setShowWhen(true)
                .addAction(0, intent.getStringExtra(EXTRA_PAUSE_LABEL), openApp("pause", 2));
        }
        builder.addAction(0, intent.getStringExtra(EXTRA_STOP_LABEL), openApp("stop", 3));
        return builder.build();
    }

    // The buttons open the app, which calls the API; that also works when
    // Android has closed the web view in the meantime.
    private PendingIntent openApp(String action, int requestCode) {
        Intent intent = new Intent(this, MainActivity.class);
        intent.setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        if (action != null) {
            intent.putExtra(FocusPlugin.EXTRA_ACTION, action);
        }
        return PendingIntent.getActivity(
            this,
            requestCode,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );
    }
}
