package pub.edholm.web.handlers

import com.google.i18n.phonenumbers.NumberParseException
import com.google.i18n.phonenumbers.PhoneNumberUtil
import com.google.i18n.phonenumbers.Phonenumber
import io.micrometer.core.instrument.MeterRegistry
import org.springframework.http.HttpStatus
import org.springframework.http.MediaType
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.server.ServerRequest
import org.springframework.web.reactive.function.server.ServerResponse
import org.springframework.web.reactive.function.server.ServerResponse.ok
import org.springframework.web.reactive.function.server.ServerResponse.status
import org.springframework.web.reactive.function.server.ServerResponse.temporaryRedirect
import org.springframework.web.reactive.function.server.body
import org.springframework.web.server.ServerWebInputException
import pub.edholm.Properties
import pub.edholm.badRequestResponse
import pub.edholm.db.Swosh
import pub.edholm.db.SwoshRepository
import pub.edholm.domain.ErrorDTO
import pub.edholm.domain.SwoshDTO
import pub.edholm.domain.SwoshUrlDTO
import pub.edholm.domain.generateUri
import pub.edholm.domain.toSwishDataDTO
import pub.edholm.domain.toSwosh
import pub.edholm.services.PreviewService
import pub.edholm.services.QRService
import reactor.core.publisher.Mono
import reactor.kotlin.core.publisher.toMono
import java.net.URI

@Component
class SwoshHandler(
  private val repo: SwoshRepository,
  private val previewService: PreviewService,
  private val qrService: QRService,
  private val properties: Properties,
  meterRegistry: MeterRegistry
) {

  private val failedCreation = meterRegistry.counter("create.failed")
  private val successCreation = meterRegistry.counter("create.success")

  fun renderIndex(req: ServerRequest): Mono<ServerResponse> =
    ok().contentType(MediaType.TEXT_HTML)
      .render("index", mapOf(Pair("user", req.principal())))

  fun renderPreview(req: ServerRequest): Mono<ServerResponse> =
    repo.findById(req.pathVariable("id"))
      .flatMap { previewService.convertToPreview(it) }
      .flatMap { preview ->
        ok().contentType(MediaType.TEXT_HTML).render(
          "preview",
          mapOf(
            Pair("swoshPreviewDTO", preview),
            Pair("scheme", properties.scheme),
            Pair("host", properties.hostname)
          )
        )
      }
      .switchIfEmpty(temporaryRedirect(URI.create("/")).build())

  fun redirectToSwish(req: ServerRequest): Mono<ServerResponse> =
    repo.findById(req.pathVariable("id"))
      .flatMap { s ->
        temporaryRedirect(s.toSwishDataDTO().generateUri())
          .build()
      }
      .switchIfEmpty(temporaryRedirect(URI.create("/")).build())

  fun renderQRCode(req: ServerRequest): Mono<ServerResponse> =
    repo.findById(req.pathVariable("id"))
      .flatMap { qrService.fetchQRCode(it) }
      .flatMap { qrCode ->
        ok().contentType(MediaType.IMAGE_PNG)
          .bodyValue(qrCode)
      }
      .switchIfEmpty(temporaryRedirect(URI.create("/")).build())

  fun createSwosh(req: ServerRequest): Mono<ServerResponse> {
    return req.bodyToMono(SwoshDTO::class.java)
      .flatMap { dto ->
        validateSwoshDTO(dto)
        constructAndInsertNewSwosh(dto)
          .flatMap { (id) ->
            successCreation.increment()
            ok()
              .contentType(MediaType.APPLICATION_JSON)
              .body(SwoshUrlDTO(id, properties.hostname, properties.scheme).toMono<SwoshUrlDTO>())
          }
      }
      .onErrorResume {
        failedCreation.increment()
        when (it) {
          is IllegalArgumentException -> ErrorDTO(reason = it.message ?: "Unknown error").badRequestResponse()
          is ServerWebInputException -> ErrorDTO(reason = "Invalid input format!").badRequestResponse()
          else -> status(HttpStatus.INTERNAL_SERVER_ERROR)
            .contentType(MediaType.APPLICATION_JSON)
            .body(ErrorDTO(reason = "Unable to generate Swosh!").toMono())
        }
      }
  }

  private fun validateSwoshDTO(dto: SwoshDTO) {
    when {
      dto.amount == null || dto.phone == null || dto.phone.isBlank() ->
        throw IllegalArgumentException("Missing input parameters. 'phone' and 'amount' is required")
      dto.amount < 1 ->
        throw IllegalArgumentException("Minimum allowed amount is 1. Got ${dto.amount}")
      dto.message != null && dto.message.length > 50 ->
        throw IllegalArgumentException("Description is too long. Max 50 chars. Got ${dto.message.length}")
      else -> validatePhoneNumber(dto.phone)
    }
  }

  private fun validatePhoneNumber(phone: String) {
    // Seems to be a valid swish number.
    if (phone.startsWith("123") && phone.length == 10) return

    val phoneUtil = PhoneNumberUtil.getInstance()
    val parsedNumber: Phonenumber.PhoneNumber?
    try {
      parsedNumber = phoneUtil.parse(phone, "SE")
    } catch (e: NumberParseException) {
      throw IllegalArgumentException("'$phone' is not a valid phone number")
    }
    require(phoneUtil.getNumberType(parsedNumber) == PhoneNumberUtil.PhoneNumberType.MOBILE) { "'$phone' is not a mobile number" }
  }

  private fun constructAndInsertNewSwosh(dto: SwoshDTO): Mono<Swosh> {
    return repo.save(dto.toSwosh())
  }

}
