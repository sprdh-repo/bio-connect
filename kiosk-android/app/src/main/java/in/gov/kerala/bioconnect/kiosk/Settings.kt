package `in`.gov.kerala.bioconnect.kiosk

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import android.util.Base64
import java.security.MessageDigest
import java.security.SecureRandom
import javax.crypto.SecretKeyFactory
import javax.crypto.spec.PBEKeySpec

data class PrinterConfig(val connection: String = "none", val address: String = "", val dpiOverride: Int = 0)

@Suppress("DEPRECATION")
class Settings(context: Context) {
    private val prefs = EncryptedSharedPreferences.create(context, "kiosk_secure",
        MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM)
    var url: String
        get() = prefs.getString("url", SitePolicy.KIOSK)!!
        set(value) { require(SitePolicy.allows(value)); prefs.edit().putString("url", value).apply() }
    var printer: PrinterConfig
        get() = PrinterConfig(prefs.getString("connection", "none")!!, prefs.getString("address", "")!!, prefs.getInt("dpi", 0))
        set(value) { prefs.edit().putString("connection", value.connection).putString("address", value.address).putInt("dpi", value.dpiOverride).apply() }
    val hasPin get() = prefs.contains("pin")
    fun setPin(pin: String) {
        require(pin.matches(Regex("[0-9]{6}")))
        val salt = ByteArray(16).also { SecureRandom().nextBytes(it) }
        prefs.edit().putString("salt", Base64.encodeToString(salt, Base64.NO_WRAP))
            .putString("pin", Base64.encodeToString(hash(pin, salt), Base64.NO_WRAP)).commit()
    }
    @Synchronized
    fun verifyPin(pin: String): Boolean {
        if (!pin.matches(Regex("[0-9]{6}")) || !hasPin) return false
        if (System.currentTimeMillis() < prefs.getLong("blockedUntil", 0)) return false
        val salt = Base64.decode(prefs.getString("salt", ""), Base64.DEFAULT)
        val correct = MessageDigest.isEqual(hash(pin, salt), Base64.decode(prefs.getString("pin", ""), Base64.DEFAULT))
        val failures = if (correct) 0 else prefs.getInt("failures", 0) + 1
        prefs.edit().putInt("failures", failures).putLong("blockedUntil", if (failures >= 5) System.currentTimeMillis() + 60_000 else 0).commit()
        if (failures >= 5) prefs.edit().putInt("failures", 0).commit()
        return correct
    }
    private fun hash(pin: String, salt: ByteArray): ByteArray {
        val spec = PBEKeySpec(pin.toCharArray(), salt, 120_000, 256)
        return try { SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).encoded } finally { spec.clearPassword() }
    }
}
