package `in`.gov.kerala.bioconnect.kiosk

import org.junit.Assert.*
import org.junit.Test

class ProtocolTest {
    private val black = 0xff000000.toInt()
    private val white = 0xffffffff.toInt()
    @Test fun packsMsbFirstAndPadsEachRow() {
        val p = intArrayOf(black, white, black, white, white, white, white, black, black,
            white, black, white, white, white, white, white, white, white)
        assertEquals("^GFA,4,4,2,A1804000", Zpl.graphic(9, 2, p))
    }
    @Test fun thresholdsLuminanceAndCompositesAlphaOnWhite() {
        val p = intArrayOf(0xff7f7f7f.toInt(), 0xff808080.toInt(), 0x00000000, 0x80000000.toInt(), 0xffff0000.toInt(), 0xff00ff00.toInt(), 0xff0000ff.toInt(), white)
        assertEquals("^GFA,1,1,1,9A", Zpl.graphic(8, 1, p))
    }
    @Test fun sendsExactlyOneLabelAndKeepsCorrectMediaLength() {
        val label = Zpl.label(2, 1, intArrayOf(black, white), 203)
        assertEquals("^XA^MNY^PW608^LL406^LH0,0^FO303,202^GFA,1,1,1,80^FS^PQ1^XZ", label)
        assertTrue(Zpl.label(2, 1, intArrayOf(black, white), 300).contains("^PW900^LL600^LH0,0^FO449,299"))
    }
    @Test fun oversizeIsCentredWithoutScaling() {
        assertTrue(Zpl.label(610, 408, IntArray(610 * 408) { white }, 203).contains("^FO0,0^GFA,30856,30856,76,"))
    }
    @Test fun permitsOnlyExactHttpsOriginPassFailPass() {
        assertTrue(SitePolicy.allows("https://reg.bioconnect.kerala.gov.in/ops"))
        for (url in listOf("http://reg.bioconnect.kerala.gov.in/kiosk", "https://reg.bioconnect.kerala.gov.in.evil.test/", "https://reg.bioconnect.kerala.gov.in@evil.test/", "https://reg.bioconnect.kerala.gov.in:444/", "intent://test", "file:///tmp/test", "javascript:alert(1)", "https://evil.test/")) assertFalse(url, SitePolicy.allows(url))
        assertTrue(SitePolicy.allows(SitePolicy.KIOSK))
    }
    @Test fun parsesBridgeContract() {
        assertEquals(PageMessage.Hello, PageMessage.parse("{\"type\":\"hello\"}"))
        assertEquals(PageMessage.Status, PageMessage.parse("{\"type\":\"status\"}"))
        assertEquals(608, (PageMessage.parse("{\"type\":\"print\",\"jobId\":\"a3f5d8c1-6559-4aa4-8e4e-31f7429e167b\",\"png\":\"YWJj\",\"widthDots\":608,\"heightDots\":406}") as PageMessage.Print).width)
    }
    @Test fun rejectsInvalidMessages() {
        for (raw in listOf("{}", "{\"type\":\"unknown\"}", "{\"type\":\"print\",\"jobId\":\"1-1-1-1-1\",\"png\":\"abc\",\"widthDots\":608,\"heightDots\":406}", "{\"type\":\"print\",\"jobId\":\"a3f5d8c1-6559-4aa4-8e4e-31f7429e167b\",\"png\":\"abc\",\"widthDots\":0,\"heightDots\":406}")) {
            assertTrue(raw, runCatching { PageMessage.parse(raw) }.isFailure)
        }
    }
    private fun hs(a: String = "000,0,0,0406,000,0,0,0,000,0,0,0", b: String = "000,0,0,0,0,0,0,0,00000000,1,000") = "\u0002$a\u0003\r\n\u0002$b\u0003\r\n\u00020000,0\u0003\r\n"
    @Test fun parsesReadyAndAllOperatorErrors() {
        assertTrue(HostStatus.parse(hs()).ready)
        assertEquals("Printer is out of labels", HostStatus.parse(hs(a="000,1,0,0406,000,0,0,0,000,0,0,0")).problem)
        assertEquals("Printer is paused", HostStatus.parse(hs(a="000,0,1,0406,000,0,0,0,000,0,0,0")).problem)
        assertEquals("Close the printer head", HostStatus.parse(hs(b="000,0,1,0,0,0,0,0,00000000,1,000")).problem)
        assertEquals("Printer is out of ribbon", HostStatus.parse(hs(b="000,0,0,1,1,0,0,0,00000000,1,000")).problem)
        assertTrue(HostStatus.parse(hs(b="000,0,0,1,0,0,0,0,00000000,1,000")).ready)
        assertEquals("Printer temperature error", HostStatus.parse(hs(a="000,0,0,0406,000,0,0,0,000,0,1,0")).problem)
        assertEquals(2, HostStatus.parse(hs(a="000,0,0,0406,002,0,0,0,000,0,0,0")).pending)
        assertTrue(runCatching { HostStatus.parse("garbage") }.isFailure)
        assertTrue(HostStatus.parse(hs()).ready)
    }
}
