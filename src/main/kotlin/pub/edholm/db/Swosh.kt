package pub.edholm.db

import com.google.zxing.EncodeHintType
import net.glxn.qrgen.core.image.ImageType
import net.glxn.qrgen.javase.QRCode
import org.springframework.data.annotation.CreatedDate
import org.springframework.data.annotation.Id
import org.springframework.data.mongodb.core.mapping.Document
import java.math.BigInteger
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
  val expiresOn: Instant? = null,
  @CreatedDate
  val createdAt: Instant = Instant.now()
) {
  companion object {
    const val DEFAULT_EXPIRY_TIME_IN_SECONDS: Long = 172800 // 2 days
    private const val ID_LENGTH = 6

    internal fun generateRandomId(): String =
      BigInteger(130, SecureRandom()).toString(32).substring(0, ID_LENGTH)
  }

  fun generateQrCodeByteArray(): ByteArray = QRCode
    .from("C${payee};${amount};${description ?: ""};4")
    .withSize(256, 256)
    .withCharset("UTF-8")
    .withHint(EncodeHintType.MARGIN, 0)
    .to(ImageType.PNG)
    .stream()
    .toByteArray()

  fun generateQrCode(): String {
    val qrCode = generateQrCodeByteArray()
    return Base64.getEncoder().encodeToString(qrCode)
  }
}
