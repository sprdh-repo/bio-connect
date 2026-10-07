package `in`.gov.kerala.bioconnect.kiosk

import android.content.Context
import android.graphics.BitmapFactory
import android.os.SystemClock
import android.util.Base64
import android.util.Log
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import org.json.JSONObject
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.CountDownLatch

data class PrinterState(val connected: Boolean = false, val name: String = "Zebra", val dpi: Int = 203,
    val connection: String = "none", val ready: Boolean = false, val problem: String = "Set up a printer", val detectedDpi: Int? = null) {
    fun json() = JSONObject().put("connected", connected).put("name", name).put("dpi", dpi)
        .put("connection", connection).put("ready", ready).put("problem", problem)
}

class PrinterService(context: Context, private val config: () -> PrinterConfig,
    private val factory: (PrinterConfig) -> PrinterTransport = { transportFor(context, it) }) {
    private val ledger = PrintLedger(context)
    private var operationDeadline = 0L
    private val mutex = Mutex()
    private val watchdog = Executors.newSingleThreadScheduledExecutor()
    @Volatile private var transport: PrinterTransport? = null
    private var activeConfig: PrinterConfig? = null
    private var detectedDpi: Int? = null
    private val mutable = MutableStateFlow(PrinterState())
    val state = mutable.asStateFlow()

    private fun checkDeadline() {
        if (SystemClock.elapsedRealtime() >= operationDeadline || Thread.currentThread().isInterrupted)
            throw java.net.SocketTimeoutException("Printer not responding")
    }
    private fun connect(c: PrinterConfig) {
        checkDeadline()
        if (transport == null || activeConfig != c) {
            disconnect()
            activeConfig = c
            val t = factory(c); transport = t
            checkDeadline()
            t.connect()
            checkDeadline()
            // Manual override remains authoritative; the query still records detected hardware DPI.
            val answer = query("! U1 getvar \"head.resolution.in_dpi\"\r\n", false, 500)
            detectedDpi = Regex("(?:203|300)").find(answer)?.value?.toInt()
            if (detectedDpi == null && c.dpiOverride == 0) error("Choose 203 or 300 dpi in printer setup")
            mutable.value = PrinterState(true, t.name, c.dpiOverride.takeIf { it == 203 || it == 300 } ?: detectedDpi!!, c.connection, false, "Checking printer", detectedDpi)
        }
    }
    private fun query(command: String, host: Boolean, timeout: Long = 1200): String {
        val t = transport ?: error("Printer is not connected")
        checkDeadline()
        t.write(command.toByteArray(Charsets.US_ASCII))
        val result = StringBuilder()
        val buffer = ByteArray(4096)
        val end = SystemClock.elapsedRealtime() + timeout
        while (SystemClock.elapsedRealtime() < end) {
            checkDeadline()
            val count = t.read(buffer, minOf(100L, end - SystemClock.elapsedRealtime()).toInt().coerceAtLeast(1))
            if (count > 0) result.append(String(buffer, 0, count, Charsets.US_ASCII))
            if (host && result.count { it == '\u0003' } >= 3) break
            if (!host && result.contains('\n')) break
            check(result.length < 16384) { "Invalid printer response" }
        }
        if (host && result.count { it == '\u0003' } < 3) error("Printer not responding")
        return result.toString()
    }
    private fun status(): HostStatus {
        val s = HostStatus.parse(query("~HS", true))
        mutable.value = mutable.value.copy(connected = true, ready = s.ready, problem = s.problem)
        return s
    }
    // A blocking Bluetooth connect/write is stopped by closing the socket, not just coroutine cancellation.
    private suspend fun <T> transaction(block: () -> T): T = withContext(Dispatchers.IO) {
        val start = SystemClock.elapsedRealtime()
        withTimeout(8000) {
            mutex.withLock {
                operationDeadline = start + 8000
                checkDeadline()
                // 0 = active, 1 = finished, 2 = watchdog owns timeout cleanup.
                val operationState = AtomicInteger(0)
                val watchdogFinished = CountDownLatch(1)
                val worker = Thread.currentThread()
                val alarm = watchdog.schedule({
                    if (operationState.compareAndSet(0, 2)) {
                        try { worker.interrupt(); transport?.close() }
                        finally { watchdogFinished.countDown() }
                    }
                }, (8000 - (SystemClock.elapsedRealtime() - start)).coerceAtLeast(1), TimeUnit.MILLISECONDS)
                try {
                    val value = block()
                    if (operationState.get() == 2) error("Printer not responding")
                    value
                } catch (e: Exception) {
                    disconnect()
                    if (operationState.get() == 2) error("Printer not responding")
                    throw e
                } finally {
                    val previous = operationState.getAndSet(1)
                    alarm.cancel(false)
                    if (previous == 2) {
                        // Do not release this pooled thread until the watchdog can no longer interrupt it.
                        while (true) {
                            try { watchdogFinished.await(); break } catch (_: InterruptedException) { /* clear and await cleanup */ }
                        }
                    }
                    Thread.interrupted()
                }
            }
        }
    }
    suspend fun refresh(): PrinterState = try {
        transaction { connect(config()); status(); mutable.value }
    } catch (e: Exception) {
        if (e is CancellationException && e !is TimeoutCancellationException) throw e
        mutable.value = mutable.value.copy(connected = false, ready = false, problem = human(e)); mutable.value
    }
    suspend fun print(job: PageMessage.Print): JSONObject {
        try {
            return transaction {
                ledger.get(job.jobId)?.let { return@transaction JSONObject(it) }
                val png = Base64.decode(job.png, Base64.DEFAULT)
                require(png.size >= 8 && png.take(8).toByteArray().contentEquals(byteArrayOf(-119, 80, 78, 71, 13, 10, 26, 10))) { "Badge is not a PNG" }
                val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
                BitmapFactory.decodeByteArray(png, 0, png.size, bounds)
                require(bounds.outWidth == job.width && bounds.outHeight == job.height) { "Badge dimensions do not match the PNG" }
                val bitmap = BitmapFactory.decodeByteArray(png, 0, png.size) ?: error("Badge image is invalid")
                val pixels = IntArray(job.width * job.height)
                try { bitmap.getPixels(pixels, 0, job.width, 0, 0, job.width, job.height) } finally { bitmap.recycle() }
                connect(config())
                val before = status()
                check(before.ready) { before.problem }
                check(before.pending == 0) { "Printer is busy; wait for the previous badge" }
                val dpi = mutable.value.dpi
                if (job.width != (if (dpi == 203) 608 else 900) || job.height != (if (dpi == 203) 406 else 600))
                    Log.w("BioConnectPrint", "Job ${job.jobId}: ${job.width}x${job.height} centred at $dpi dpi without scaling")
                val zpl = Zpl.label(job.width, job.height, pixels, dpi)
                // Persist BEFORE writing: a crash or timeout can never silently re-send the same badge.
                remember(job.jobId, reply(job.jobId, false, "Print outcome uncertain; check the printer before trying again"))
                checkDeadline()
                transport!!.write(zpl.toByteArray(Charsets.US_ASCII))
                // This reports printer acceptance/readiness, not an optical confirmation of a physical label.
                var after = status()
                while (after.ready && after.pending > 0) { Thread.sleep(100); after = status() }
                check(after.ready) { after.problem }
                reply(job.jobId, true, "Badge sent to printer").also { remember(job.jobId, it) }
            }
        } catch (e: Exception) {
            if (e is CancellationException && e !is TimeoutCancellationException) throw e
            val message = human(e)
            mutable.value = mutable.value.copy(connected = false, ready = false, problem = message)
            return reply(job.jobId, false, message)
        }
    }
    suspend fun calibrate(): String = try { transaction { connect(config()); transport!!.write("~JC".toByteArray()); "Calibration started" } }
        catch (e: Exception) { human(e) }
    private fun remember(id: String, response: JSONObject) {
        ledger.put(id, response.toString())
    }
    private fun reply(id: String, ok: Boolean, message: String) = JSONObject().put("type", "printed").put("jobId", id).put("ok", ok).put("message", message)
    private fun human(e: Exception): String = when (e) {
        is TimeoutCancellationException, is java.net.SocketTimeoutException, is InterruptedException -> "Printer not responding"
        is SecurityException -> "Allow Bluetooth, USB or local network access in setup"
        is java.io.IOException -> "Printer is not connected"
        else -> e.message?.take(120) ?: "Printer not responding"
    }
    private fun disconnect() { transport?.close(); transport = null; activeConfig = null; detectedDpi = null }
    fun close() { transport?.close(); watchdog.shutdownNow(); ledger.close() }
}
