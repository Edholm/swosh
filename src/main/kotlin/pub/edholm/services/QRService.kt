package pub.edholm.services

import com.google.zxing.EncodeHintType
import net.glxn.qrgen.core.image.ImageType
import net.glxn.qrgen.javase.QRCode
import org.slf4j.LoggerFactory
import org.springframework.cache.annotation.Cacheable
import org.springframework.http.HttpHeaders
import org.springframework.http.MediaType
import org.springframework.stereotype.Service
import org.springframework.web.reactive.function.BodyInserters
import org.springframework.web.reactive.function.client.WebClient
import pub.edholm.Properties
import pub.edholm.db.Swosh
import pub.edholm.domain.IntValue
import pub.edholm.domain.StringValue
import pub.edholm.dto.SwishQRInputDTO
import reactor.core.publisher.Mono
import java.util.*

@Service
class QRService(props: Properties) {
  private val logger = LoggerFactory.getLogger(QRService::class.java)
  private val webClient = WebClient.builder()
    .baseUrl(props.swishQrEndpoint)
    .defaultHeader(HttpHeaders.CONTENT_TYPE, MediaType.APPLICATION_JSON_VALUE)
    .build()

  fun generateQRCode(swosh: Swosh): ByteArray = QRCode
    .from("C${swosh.payee};${swosh.amount};${swosh.description ?: ""};4")
    .withSize(256, 256)
    .withCharset("UTF-8")
    .withHint(EncodeHintType.MARGIN, 0)
    .to(ImageType.PNG)
    .stream()
    .toByteArray()

  @Cacheable("swishQRCode")
  fun fetchQRCode(swosh: Swosh): Mono<ByteArray> {
    logger.debug("Fetching QR code from Swish for {}", swosh.id)
    val qrInput =
      SwishQRInputDTO(
        payee = StringValue(swosh.payee),
        amount = IntValue(swosh.amount),
        message = StringValue(swosh.description ?: ""),
        transparent = true
      )

    return webClient.post()
      .body(BodyInserters.fromValue(qrInput))
      .retrieve()
      .bodyToMono(ByteArray::class.java)
  }

  fun convertToBase64(qrCode: ByteArray): String {
    return Base64.getEncoder().encodeToString(qrCode)
  }
}