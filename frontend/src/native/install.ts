import {Capacitor} from '@capacitor/core'

import {createNativeAuthBridge} from './nativeAuth'

// Imported first in main.ts: the login page, the base store and the token
// refresh decide at start-up whether they run inside an app shell.
if (Capacitor.isNativePlatform() && !window.vikunjaDesktop) {
	window.vikunjaDesktop = createNativeAuthBridge()
}
