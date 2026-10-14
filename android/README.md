# Hefzul Tasks for Android

A [Capacitor](https://capacitorjs.com/) shell around the web app in `../frontend`. The APK carries the built web app, so it opens without a network and shows the tasks, habits and countdowns cached on the phone. Edits made offline wait in the outbox and sync when the phone is back online.

What the shell adds to the web app:

- **Server choice on first launch.** One APK works with any Hefzul Tasks server. The app asks for the server URL, then opens the server's sign-in page in the browser. After sign-in the browser hands a code back to the app (OAuth with PKCE, redirect `vikunja-hefzul://callback`). The app keeps the OAuth refresh token, because the web app's refresh cookie cannot reach it from another origin.
- **Focus timer notification.** While a focus session runs, an ongoing notification counts down the time left and has Pause / Resume and Stop buttons (a foreground service, `FocusService.java`). The buttons open the app, which sends the change to the server.
- **End-of-session alarm.** The end of each session is scheduled as a local notification, so it rings even if Android has closed the app.
- **Habit reminders.** The app schedules the next seven days of habit reminders on the phone and replans them whenever the habit list changes. Web Push does not reach an app, so the push settings page is hidden here. Reminder times use the phone's time zone; keep it the same as the time zone in your Hefzul Tasks settings.
- **Updates.** On every start the app reads `/downloads/android/latest.json` from the server and offers the newer APK when there is one.

## Layout

| Path | What |
|------|------|
| `package.json`, `capacitor.config.json` | Capacitor project; the web app is taken from `../frontend/dist` |
| `native/` | The Android Studio / Gradle project |
| `native/app/src/main/java/com/hefzul/tasks/` | `MainActivity`, the `HefzulFocus` plugin and `FocusService` |
| `../frontend/src/native/` | The web side: sign-in bridge, focus notification, habit reminders, update check |

The web code only uses the native parts when it runs inside the app (`isAndroidApp()`), so the same build serves the browser.

## Build

The [`hefzul-android`](../.github/workflows/hefzul-android.yml) workflow builds the APK on every push to `main` that touches `frontend/` or `android/`, and attaches it to a release for tags `hefzul-v*`. Pull requests that touch `android/` or `frontend/src/native/` check that it builds.

To build on your own machine you need Node 24 with pnpm, JDK 21 and the Android SDK (platform 36), for example from Android Studio.

```sh
cd android
pnpm install
pnpm run build:web       # builds ../frontend/dist
pnpm exec cap sync android
cd native
./gradlew assembleRelease   # app/build/outputs/apk/release/app-release.apk
```

To run it on a phone or emulator from Android Studio: `pnpm exec cap open android`.

Without the signing variables below, a local release build is signed with your debug key and gets version code 1.

### Version

The workflow sets `HEFZUL_VERSION_CODE` to the run number plus 100 and `HEFZUL_VERSION_NAME` to the web app version plus that code, for example `2.7.0-142`. Android only installs an update with a higher version code.

### Signing

Android only installs an update signed with the same key as the installed app. Create one key and keep it safe (a lost key means uninstalling the app, and its offline data, on every phone):

```sh
keytool -genkeypair -v -keystore hefzul.jks -alias hefzul -keyalg RSA -keysize 4096 -validity 10000
base64 -w0 hefzul.jks   # the value for HEFZUL_KEYSTORE_BASE64
```

Add these repository secrets (Settings → Secrets and variables → Actions):

| Secret | Value |
|--------|-------|
| `HEFZUL_KEYSTORE_BASE64` | The keystore, base64-encoded |
| `HEFZUL_KEYSTORE_PASSWORD` | Its password |
| `HEFZUL_KEY_ALIAS` | `hefzul` (the alias above) |
| `HEFZUL_KEY_PASSWORD` | The key password |

Without them the workflow signs with the runner's throwaway debug key and warns. Such an APK installs on a clean phone, but the next build will not install over it.

## Install and update

See [deploy/README.md](../deploy/README.md#8-android-app): copy `hefzul-tasks.apk` and `latest.json` from the workflow run into `deploy/downloads/android/` on the server, then open the APK link on the phone.

The server must allow the app's origin, `https://localhost`, through CORS. The deploy setup does this by default (`HEFZUL_CORS_ORIGINS`).

## Permissions

| Permission | Why |
|------------|-----|
| Notifications | Focus timer, end-of-session alarm and habit reminders; asked the first time one is needed |
| Alarms (`USE_EXACT_ALARM`) | The end-of-session alarm rings on time. Granted at install; the APK is not distributed through Google Play |
| Foreground service | Keeps the focus timer notification while a session runs |
| Internet | Talks to your server |

## Limitations

- Sign-in always goes through the browser, also with a password, because the app cannot hold the server's sign-in cookie.
- If Android has closed the app, a Pause or Stop button press opens it first.
- Web Push is not used in the app; habit reminders are planned on the phone and need the app to be opened at least once a week.
