package `in`.gov.kerala.bioconnect.kiosk

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Color
import android.os.SystemClock
import android.util.Base64
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import kotlinx.coroutines.runBlocking
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.util.UUID
import java.util.concurrent.CountDownLatch
import java.util.concurrent.atomic.AtomicInteger

@RunWith(AndroidJUnit4::class)
class PrinterSafetyTest {
    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext
    private fun job(): PageMessage.Print {
        val b = Bitmap.createBitmap(8, 1, Bitmap.Config.ARGB_8888).apply { eraseColor(Color.BLACK) }
        val out = ByteArrayOutputStream(); b.compress(Bitmap.CompressFormat.PNG, 100, out); b.recycle()
        return PageMessage.Print(UUID.randomUUID().toString(), Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP), 8, 1)
    }
    private open class Fake : PrinterTransport {
        override val name = "Fake Zebra"
        var labels = 0
        var response = ByteArray(0)
        override fun connect() = Unit
        override fun close() = Unit
        override fun write(bytes: ByteArray) {
            val s = String(bytes)
            if (s.startsWith("^XA")) labels++
            else response = if (s.startsWith("! U1")) "\"203\"\r\n".toByteArray() else
                "\u0002000,0,0,0406,000,0,0,0,000,0,0,0\u0003\r\n\u0002000,0,0,0,0,0,0,0,00000000,1,000\u0003\r\n\u00020000,0\u0003\r\n".toByteArray()
        }
        override fun read(buffer: ByteArray, timeoutMs: Int): Int { val n = response.size; response.copyInto(buffer); response = ByteArray(0); return n }
    }
    @Test fun blockedConnectionTimesOutThenNextJobReconnects() = runBlocking {
        val gate = CountDownLatch(1)
        val stuck = object : Fake() {
            override fun connect() { gate.await(); throw IOException("Closed") }
            override fun close() { gate.countDown() }
        }
        val recovered = Fake()
        val attempts = AtomicInteger()
        val service = PrinterService(context, { PrinterConfig("tcp", "127.0.0.1", 203) }, { if (attempts.getAndIncrement() == 0) stuck else recovered })
        val start = SystemClock.elapsedRealtime()
        val result = service.print(job())
        assertFalse(result.getBoolean("ok"))
        assertEquals("Printer not responding", result.getString("message"))
        assertTrue("Elapsed ${SystemClock.elapsedRealtime() - start}", SystemClock.elapsedRealtime() - start < 9000)
        assertEquals(0, stuck.labels)
        assertTrue(service.print(job()).getBoolean("ok"))
        assertEquals(1, recovered.labels)
        service.close()
    }
    @Test fun interruptedWriteCannotReplayEvenAfterServiceRestart() = runBlocking {
        val fake = object : Fake() {
            override fun write(bytes: ByteArray) { super.write(bytes); if (String(bytes).startsWith("^XA")) throw IOException("Write outcome unknown") }
        }
        val j = job()
        val first = PrinterService(context, { PrinterConfig("tcp", "127.0.0.1", 203) }, { fake })
        assertFalse(first.print(j).getBoolean("ok")); first.close()
        val second = PrinterService(context, { PrinterConfig("tcp", "127.0.0.1", 203) }, { fake })
        assertTrue(second.print(j).getString("message").contains("uncertain"))
        assertEquals(1, fake.labels); second.close()
    }
    @Test fun eventLedgerDoesNotEvictOlderOrUncertainReceipts() {
        val ids = (0..150).map { UUID.randomUUID().toString() }
        PrintLedger(context).use { ledger -> ids.forEach { ledger.put(it, "uncertain") } }
        PrintLedger(context).use { ledger -> assertEquals("uncertain", ledger.get(ids.first())) }
    }
}
