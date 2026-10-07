package `in`.gov.kerala.bioconnect.kiosk

import android.Manifest
import android.content.Context
import android.os.SystemClock
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import androidx.test.core.app.ActivityScenario
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.runBlocking
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import java.io.ByteArrayInputStream
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

@RunWith(AndroidJUnit4::class)
class BridgeSmokeTest {
    private class FakeTransport : PrinterTransport {
        override val name = "ZD421 fake"
        var connected = false
        val labels = mutableListOf<String>()
        private var answer = ByteArray(0)
        var fault = false
        override fun connect() { connected = true }
        override fun close() { connected = false }
        override fun write(bytes: ByteArray) {
            val s = String(bytes)
            when {
                s.startsWith("! U1") -> answer = "\"203\"\r\n".toByteArray()
                s == "~HS" -> answer = ("\u0002000,${if (fault) 1 else 0},0,0406,000,0,0,0,000,0,0,0\u0003\r\n" +
                    "\u0002000,0,0,0,0,0,0,0,00000000,1,000\u0003\r\n\u00020000,0\u0003\r\n").toByteArray()
                s.startsWith("^XA") -> labels.add(s)
            }
        }
        override fun read(buffer: ByteArray, timeoutMs: Int): Int { val n = minOf(buffer.size, answer.size); answer.copyInto(buffer, 0, 0, n); answer = answer.copyOfRange(n, answer.size); return n }
    }
    @Test fun localFixturePrintsOnceRepliesAndRejectsOtherOrigins() {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        instrumentation.uiAutomation.executeShellCommand("pm grant ${context.packageName} ${Manifest.permission.CAMERA}").close()
        val fake = FakeTransport()
        val service = PrinterService(context, { PrinterConfig("tcp", "127.0.0.1", 203) }, { fake })
        val html = instrumentation.context.assets.open("bridge.html").bufferedReader().readText()
        lateinit var web: WebView
        ActivityScenario.launch(BridgeFixtureActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                web = WebView(activity).apply {
                    settings.javaScriptEnabled = true
                    webViewClient = object : WebViewClient() {
                        override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest) =
                            WebResourceResponse("text/html", "UTF-8", ByteArrayInputStream(html.toByteArray()))
                    }
                }
                activity.setContentView(web)
                assertTrue(KioskBridge.install(web, service, activity.lifecycleScope))
                web.loadUrl("${SitePolicy.ORIGIN}/bridge-fixture")
            }
            fun js(expression: String): String {
                val latch = CountDownLatch(1); var result = ""
                scenario.onActivity { web.evaluateJavascript(expression) { result = it; latch.countDown() } }
                assertTrue(latch.await(5, TimeUnit.SECONDS)); return result
            }
            fun awaitJs(expression: String) {
                val end = SystemClock.elapsedRealtime() + 15000
                while (SystemClock.elapsedRealtime() < end) { if (js(expression) == "true") return; SystemClock.sleep(100) }
                fail("Fixture condition was not met: $expression")
            }
            awaitJs("responses.some(r => r.type === 'info' && r.printer.dpi === 203 && r.printer.ready)")
            val id = java.util.UUID.randomUUID().toString()
            js("printBadge('$id')")
            awaitJs("responses.some(r => r.type === 'printed' && r.ok)")
            assertEquals(1, fake.labels.size)
            assertTrue(fake.labels.single().contains("^GFA,30856,30856,76,FF"))
            assertTrue(fake.labels.single().endsWith("^FS^PQ1^XZ"))
            js("printBadge('$id')")
            awaitJs("responses.filter(r => r.type === 'printed').length === 2")
            assertEquals(1, fake.labels.size)
            fake.fault = true
            js("printBadge('${java.util.UUID.randomUUID()}')")
            awaitJs("responses.some(r => r.message === 'Printer is out of labels')")
            assertEquals(1, fake.labels.size)
            scenario.onActivity { web.loadUrl("https://evil.test/bridge-fixture") }
            awaitJs("location.hostname === 'evil.test' && typeof BioConnectKiosk === 'undefined'")
            fake.fault = false
            scenario.onActivity { web.loadUrl("${SitePolicy.ORIGIN}/bridge-fixture") }
            awaitJs("responses.some(r => r.type === 'info' && r.printer.ready)")
            scenario.onActivity { web.destroy() }
        }
        service.close()
    }
    @Test fun knownPngDecodesAndTransparentPixelsStayWhite() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val b = android.graphics.Bitmap.createBitmap(9, 2, android.graphics.Bitmap.Config.ARGB_8888)
        b.setPixel(0, 0, android.graphics.Color.BLACK); b.setPixel(8, 1, android.graphics.Color.BLACK)
        val out = java.io.ByteArrayOutputStream(); b.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, out); b.recycle()
        val fake = FakeTransport()
        val service = PrinterService(context, { PrinterConfig("tcp", "127.0.0.1", 203) }, { fake })
        val job = PageMessage.Print(java.util.UUID.randomUUID().toString(), android.util.Base64.encodeToString(out.toByteArray(), android.util.Base64.NO_WRAP), 9, 2)
        val response = runBlocking { service.print(job) }
        assertTrue(response.toString(), response.getBoolean("ok"))
        assertTrue(fake.labels.single().contains("^GFA,4,4,2,80000080"))
        service.close()
    }
}
