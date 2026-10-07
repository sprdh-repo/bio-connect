package `in`.gov.kerala.bioconnect.kiosk

import android.app.admin.DeviceAdminReceiver
import android.app.admin.DevicePolicyManager
import android.content.*
import android.os.Build

class KioskDeviceAdminReceiver : DeviceAdminReceiver() {
    override fun onEnabled(context: Context, intent: Intent) { configureOwner(context) }
}

fun configureOwner(context: Context) {
    val dpm = context.getSystemService(DevicePolicyManager::class.java)
    if (!dpm.isDeviceOwnerApp(context.packageName)) return
    val admin = ComponentName(context, KioskDeviceAdminReceiver::class.java)
    dpm.setLockTaskPackages(admin, arrayOf(context.packageName))
    if (Build.VERSION.SDK_INT >= 28) dpm.setLockTaskFeatures(admin, DevicePolicyManager.LOCK_TASK_FEATURE_NONE)
    val filter = IntentFilter(Intent.ACTION_MAIN).apply { addCategory(Intent.CATEGORY_HOME); addCategory(Intent.CATEGORY_DEFAULT) }
    dpm.addPersistentPreferredActivity(admin, filter, ComponentName(context, MainActivity::class.java))
}

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return
        configureOwner(context)
        // Device owners are exempt from background activity launch restrictions.
        // On unmanaged tablets the OS may suppress this; select this app as the default Home.
        context.startActivity(Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    }
}
