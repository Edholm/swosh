package pub.edholm.domain

import java.time.Instant

data class SwoshPreviewDTO(
  val id: String,
  val payee: String,
  val amount: Int,
  val description: String?,
  val expiresOn: Instant?,
  val swishUri: String,
  val qrCode: String
)