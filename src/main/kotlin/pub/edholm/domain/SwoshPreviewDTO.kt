package pub.edholm.domain

import pub.edholm.db.Swosh
import java.time.Instant

data class SwoshPreviewDTO(
  val id: String,
  val payee: String,
  val amount: Int,
  val description: String?,
  val expiresOn: Instant?,
  val swishUri: String,
  val qrCode: String,
  val createdAt: Instant?
) {
  companion object {
    fun valueOf(swosh: Swosh): SwoshPreviewDTO {
      val swishUri = swosh.toSwishDataDTO().generateUri()
      return SwoshPreviewDTO(
        swosh.id, swosh.payee, swosh.amount, swosh.description,
        swosh.expiresOn, swishUri.toASCIIString(), swosh.generateQrCode(swishUri),
        swosh.createdAt
      )
    }
  }
}