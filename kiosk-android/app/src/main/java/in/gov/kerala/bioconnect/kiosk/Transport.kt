package `in`.gov.kerala.bioconnect.kiosk

import android.annotation.SuppressLint
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothSocket
import android.content.Context
import android.hardware.usb.*
import java.io.InputStream
import java.net.InetSocketAddress
import java.net.Socket
import java.util.UUID

interface PrinterTransport {
    val name: String
    fun connect()
    fun write(bytes: ByteArray)
    fun read(buffer: ByteArray, timeoutMs: Int): Int
    fun close()
}

class TcpTransport(private val ip: String) : PrinterTransport {
    @Volatile private var socket: Socket? = null
    @Volatile private var closed = false
    override val name = "Zebra ($ip)"
    override fun connect() {
        // Literal IP only: no unbounded DNS operation in the job deadline.
        require(ip.matches(Regex("(?:[0-9]{1,3}\\.){3}[0-9]{1,3}")) && ip.split('.').all { it.toInt() in 0..255 }) { "Enter a valid IPv4 printer address" }
        val s = Socket(); socket = s
        if (closed) { s.close(); error("Printer not responding") }
        s.tcpNoDelay = true
        s.connect(InetSocketAddress(ip, 9100), 1800)
    }
    override fun write(bytes: ByteArray) { socket!!.getOutputStream().apply { write(bytes); flush() } }
    override fun read(buffer: ByteArray, timeoutMs: Int): Int {
        socket!!.soTimeout = timeoutMs
        return try { socket!!.getInputStream().read(buffer).also { if (it < 0) error("Printer disconnected") } } catch (_: java.net.SocketTimeoutException) { 0 }
    }
    override fun close() { closed = true; runCatching { socket?.close() }; socket = null }
}

@SuppressLint("MissingPermission")
class BluetoothTransport(context: Context, private val address: String) : PrinterTransport {
    private val adapter = context.getSystemService(BluetoothManager::class.java).adapter
    @Volatile private var socket: BluetoothSocket? = null
    @Volatile private var closed = false
    override val name get() = runCatching { adapter.getRemoteDevice(address).name ?: "Zebra" }.getOrDefault("Zebra")
    override fun connect() {
        check(adapter != null && adapter.isEnabled) { "Turn on Bluetooth" }
        val s = adapter.getRemoteDevice(address).createRfcommSocketToServiceRecord(UUID.fromString("00001101-0000-1000-8000-00805f9b34fb"))
        socket = s
        if (closed) { s.close(); error("Printer not responding") }
        s.connect()
    }
    override fun write(bytes: ByteArray) { socket!!.outputStream.apply { write(bytes); flush() } }
    override fun read(buffer: ByteArray, timeoutMs: Int): Int = poll(socket!!.inputStream, buffer, timeoutMs)
    override fun close() { closed = true; runCatching { socket?.close() }; socket = null }
    private fun poll(input: InputStream, buffer: ByteArray, timeout: Int): Int {
        val end = System.nanoTime() + timeout * 1_000_000L
        while (System.nanoTime() < end) {
            val available = input.available()
            if (available > 0) return input.read(buffer, 0, minOf(buffer.size, available))
            Thread.sleep(10)
        }
        return 0
    }
}

class UsbTransport(context: Context, private val address: String) : PrinterTransport {
    private val manager = context.getSystemService(UsbManager::class.java)
    @Volatile private var connection: UsbDeviceConnection? = null
    @Volatile private var closed = false
    private var iface: UsbInterface? = null
    private var output: UsbEndpoint? = null
    private var input: UsbEndpoint? = null
    override val name get() = device()?.productName ?: "Zebra USB"
    private fun device(): UsbDevice? = manager.deviceList.values.firstOrNull { usbKey(it) == address }
    override fun connect() {
        val d = device() ?: error("Connect the USB printer")
        check(manager.hasPermission(d)) { "Allow USB printer access in setup" }
        // Prefer the USB printer interface, never claim an unrelated HID interface.
        val interfaces = (0 until d.interfaceCount).map { d.getInterface(it) }.sortedBy { if (it.interfaceClass == UsbConstants.USB_CLASS_PRINTER) 0 else 1 }
        val pair = interfaces.firstOrNull { i ->
            (0 until i.endpointCount).map { i.getEndpoint(it) }.any { it.type == UsbConstants.USB_ENDPOINT_XFER_BULK && it.direction == UsbConstants.USB_DIR_OUT } &&
                (0 until i.endpointCount).map { i.getEndpoint(it) }.any { it.type == UsbConstants.USB_ENDPOINT_XFER_BULK && it.direction == UsbConstants.USB_DIR_IN }
        } ?: error("USB printer has no bidirectional bulk interface")
        val c = manager.openDevice(d) ?: error("USB printer is unavailable")
        connection = c
        if (closed) { c.close(); error("Printer not responding") }
        check(c.claimInterface(pair, true)) { "USB printer is busy" }
        iface = pair
        for (e in (0 until pair.endpointCount).map { pair.getEndpoint(it) }) {
            if (e.type == UsbConstants.USB_ENDPOINT_XFER_BULK) {
                if (e.direction == UsbConstants.USB_DIR_OUT) output = e else input = e
            }
        }
    }
    override fun write(bytes: ByteArray) {
        var offset = 0
        while (offset < bytes.size) {
            val n = connection!!.bulkTransfer(output, bytes, offset, minOf(16384, bytes.size - offset), 1000)
            check(n > 0) { "Printer not responding" }
            offset += n
        }
    }
    override fun read(buffer: ByteArray, timeoutMs: Int): Int = connection!!.bulkTransfer(input, buffer, buffer.size, timeoutMs).coerceAtLeast(0)
    override fun close() {
        closed = true
        val c = connection; connection = null
        runCatching { iface?.let { c?.releaseInterface(it) } }
        runCatching { c?.close() }; iface = null; input = null; output = null
    }
    companion object {
        // Stable across reboots and USB bus re-enumeration; provision one printer per tablet.
        fun usbKey(device: UsbDevice) = "${device.vendorId}:${device.productId}"
    }
}

fun transportFor(context: Context, config: PrinterConfig): PrinterTransport = when (config.connection) {
    "bluetooth" -> BluetoothTransport(context, config.address)
    "usb" -> UsbTransport(context, config.address)
    "tcp" -> {
        if (android.os.Build.VERSION.SDK_INT >= 37 && androidx.core.content.ContextCompat.checkSelfPermission(context,
                android.Manifest.permission.ACCESS_LOCAL_NETWORK) != android.content.pm.PackageManager.PERMISSION_GRANTED)
            throw SecurityException("Local network permission is required")
        TcpTransport(config.address)
    }
    else -> error("Set up a printer in the admin menu")
}
