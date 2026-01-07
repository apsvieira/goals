import { PushNotifications, type Token, type PushNotificationSchema, type ActionPerformed } from '@capacitor/push-notifications';
import { Capacitor } from '@capacitor/core';
import { registerDevice, unregisterDevice } from './api';

// Store the device ID returned from backend for unregistration
let currentDeviceId: string | null = null;

/**
 * Initialize push notifications for native platforms
 * Should be called after successful authentication
 */
export async function initPushNotifications(): Promise<void> {
  // Only run on native platforms (iOS/Android)
  if (!Capacitor.isNativePlatform()) {
    console.log('[Push] Skipping push notification init - not a native platform');
    return;
  }

  try {
    // Request permission
    const permResult = await PushNotifications.requestPermissions();

    if (permResult.receive === 'granted') {
      // Register with APNs/FCM
      await PushNotifications.register();
    } else {
      console.log('[Push] Permission denied:', permResult.receive);
      return;
    }

    // Handle registration success - get the FCM/APNs token
    await PushNotifications.addListener('registration', async (token: Token) => {
      console.log('[Push] Registration successful, token:', token.value.substring(0, 20) + '...');

      // Determine platform
      const platform = Capacitor.getPlatform(); // 'ios' or 'android'

      try {
        // Register the token with our backend
        const device = await registerDevice(token.value, platform);
        currentDeviceId = device.id;
        console.log('[Push] Device registered with backend, ID:', device.id);
      } catch (error) {
        console.error('[Push] Failed to register device with backend:', error);
      }
    });

    // Handle registration errors
    await PushNotifications.addListener('registrationError', (error) => {
      console.error('[Push] Registration failed:', error);
    });

    // Handle push notification received while app is in foreground
    await PushNotifications.addListener('pushNotificationReceived', (notification: PushNotificationSchema) => {
      console.log('[Push] Notification received in foreground:', notification);
      // Could show an in-app notification banner here
    });

    // Handle user tapping on a push notification
    await PushNotifications.addListener('pushNotificationActionPerformed', (action: ActionPerformed) => {
      console.log('[Push] Notification action performed:', action);
      // Could navigate to specific screen based on notification data
      // action.notification.data contains custom payload from backend
    });

  } catch (error) {
    console.error('[Push] Initialization failed:', error);
  }
}

/**
 * Unregister push notifications
 * Should be called when user logs out
 */
export async function unregisterPushNotifications(): Promise<void> {
  if (!Capacitor.isNativePlatform()) {
    return;
  }

  try {
    // Unregister from backend first
    if (currentDeviceId) {
      try {
        await unregisterDevice(currentDeviceId);
        console.log('[Push] Device unregistered from backend');
      } catch (error) {
        console.error('[Push] Failed to unregister device from backend:', error);
      }
      currentDeviceId = null;
    }

    // Remove all listeners
    await PushNotifications.removeAllListeners();

    console.log('[Push] Push notifications unregistered');
  } catch (error) {
    console.error('[Push] Unregistration failed:', error);
  }
}
