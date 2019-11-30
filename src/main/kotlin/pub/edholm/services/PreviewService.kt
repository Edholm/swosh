package pub.edholm.services

import org.springframework.stereotype.Service
import pub.edholm.db.Swosh
import pub.edholm.domain.SwoshPreviewDTO
import reactor.core.publisher.Mono

@Service
class PreviewService(private val qrService: QRService) {
  fun convertToPreview(swosh: Swosh, fetchQRCode: Boolean = true): Mono<SwoshPreviewDTO> {
    return if (fetchQRCode) {
      qrService.fetchQRCode(swosh)
        .map { qrCode -> SwoshPreviewDTO.valueOf(swosh, qrService.convertToBase64(qrCode)) }
    } else {
      Mono.just(SwoshPreviewDTO.valueOf(swosh, qrService.convertToBase64(qrService.generateQRCode(swosh))))
    }
  }
}