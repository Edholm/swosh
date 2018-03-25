package pub.edholm.web.handlers

import com.google.zxing.EncodeHintType
import net.glxn.qrgen.core.image.ImageType
import net.glxn.qrgen.javase.QRCode
import org.springframework.data.annotation.Id
import org.springframework.data.mongodb.core.mapping.Document
import java.math.BigInteger
import java.net.URI
import java.security.SecureRandom
import java.time.Instant
import java.util.*

@Document
data class Swosh(
  @Id
  val id: String = generateRandomId(),
  val payee: String = "N/A",
  val amount: Int = 1,
  val description: String? = null,
  val expiresOn: Instant? = null
) {
  companion object {
    const val DEFAULT_EXPIRY_TIME_IN_SECONDS: Long = 172800 // 2 days
    private const val ID_LENGTH = 6

    internal fun generateRandomId(): String =
      BigInteger(130, SecureRandom()).toString(32).substring(0, ID_LENGTH)
  }

  fun generateQrCode(swishUri: URI): String {
    val qrCode = QRCode
      .from(swishUri.toASCIIString())
      .withSize(256, 256)
      .withCharset("UTF-8")
      .withHint(EncodeHintType.MARGIN, 0)
      .to(ImageType.PNG)
      .stream()
    return Base64.getEncoder().encodeToString(qrCode.toByteArray())
  }
}
