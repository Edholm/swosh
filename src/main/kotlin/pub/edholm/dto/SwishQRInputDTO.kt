package pub.edholm.dto

import pub.edholm.domain.IntValue
import pub.edholm.domain.StringValue

data class SwishQRInputDTO(
  val payee: StringValue,
  val amount: IntValue,
  val message: StringValue,
  val format: Format = Format.png,
  val size: Int = 300,
  val border: Int = 2,
  val transparent: Boolean = false
) {
  enum class Format {
    jpg, png, svg
  }
}
