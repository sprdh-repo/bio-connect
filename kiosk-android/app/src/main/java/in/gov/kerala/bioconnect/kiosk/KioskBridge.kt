package `in`.gov.kerala.bioconnect.kiosk

import android.webkit.WebView
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import org.json.JSONObject

object KioskBridge {
    fun install(webView: WebView, service: PrinterService, scope: CoroutineScope): Boolean {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return false
        WebViewCompat.addWebMessageListener(webView, "BioConnectKiosk", setOf(SitePolicy.ORIGIN)) { _, message, origin, mainFrame, reply ->
            // Origin rules protect injection; this additionally rejects messages from embedded frames.
            if (!mainFrame || !SitePolicy.allows(origin.toString())) return@addWebMessageListener
            val raw = message.data ?: return@addWebMessageListener
            scope.launch {
                val response = try {
                    when (val parsed = PageMessage.parse(raw)) {
                        PageMessage.Hello, PageMessage.Status -> JSONObject().put("type", "info")
                            .put("app", BuildConfig.VERSION_NAME).put("printer", service.refresh().json())
                        is PageMessage.Print -> service.print(parsed)
                    }
                } catch (e: Exception) {
                    if (e is kotlinx.coroutines.CancellationException) throw e
                    val id = runCatching { JSONObject(raw).optString("jobId").take(80) }.getOrDefault("")
                    JSONObject().put("type", "printed").put("jobId", id).put("ok", false)
                        .put("message", e.message?.take(120) ?: "Invalid kiosk message")
                }
                runCatching { reply.postMessage(response.toString()) }.onFailure {
                    // A navigation or renderer loss can destroy the reply target after a job finishes.
                    android.util.Log.w("BioConnectBridge", "Page unavailable for job reply")
                }
            }
        }
        return true
    }
}
