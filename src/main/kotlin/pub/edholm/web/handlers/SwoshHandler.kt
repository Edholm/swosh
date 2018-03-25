package pub.edholm.web.handlers

import com.google.i18n.phonenumbers.NumberParseException
import com.google.i18n.phonenumbers.PhoneNumberUtil
import com.google.i18n.phonenumbers.Phonenumber
import org.springframework.http.HttpStatus
import org.springframework.http.MediaType
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.server.ServerRequest
import org.springframework.web.reactive.function.server.ServerResponse
import org.springframework.web.reactive.function.server.ServerResponse.ok
import org.springframework.web.reactive.function.server.ServerResponse.status
import org.springframework.web.reactive.function.server.ServerResponse.temporaryRedirect
import org.springframework.web.reactive.function.server.body
import pub.edholm.badRequestResponse
import pub.edholm.domain.*
import reactor.core.publisher.Mono
import reactor.core.publisher.toMono
import java.net.URI

@Component
class SwoshHandler(private val repo: SwoshRepository) {
  fun renderIndex(req: ServerRequest): Mono<ServerResponse> =
    ok().contentType(MediaType.TEXT_HTML)
      .render("index", mapOf(Pair("user", req.principal())))

  fun renderPreview(req: ServerRequest) =
    repo.findById(req.pathVariable("id"))
      .flatMap { swosh ->
        val swoshPreviewDTO = SwoshPreviewDTO.valueOf(swosh)
        ok().contentType(MediaType.TEXT_HTML).render("preview", swoshPreviewDTO)
      }
      .switchIfEmpty(temporaryRedirect(URI.create("/")).build())

  fun redirectToSwish(req: ServerRequest) =
    repo.findById(req.pathVariable("id"))
      .flatMap { s ->
        temporaryRedirect(s.toSwishDataDTO().generateUri())
          .build()
      }
      .switchIfEmpty(temporaryRedirect(URI.create("/")).build())

  fun createSwosh(req: ServerRequest): Mono<ServerResponse> {
    return req.bodyToMono(SwoshDTO::class.java)
      .flatMap { dto ->
        val swoshErrorDTO = validateSwoshDTO(dto)
        when {
          swoshErrorDTO != null -> swoshErrorDTO.badRequestResponse()
          else ->
            constructAndInsertNewSwosh(dto)
              .flatMap { (id) ->
                ok()
                  .contentType(MediaType.APPLICATION_JSON_UTF8)
                  .body(SwoshUrlDTO(id).toMono())
              }
              .onErrorResume {
                status(HttpStatus.INTERNAL_SERVER_ERROR)
                  .contentType(MediaType.APPLICATION_JSON_UTF8)
                  .body(ErrorDTO(reason = "Unable to generate Swosh!").toMono())
              }
        }
      }
      .onErrorResume {
        ErrorDTO(reason = "Invalid input format!").badRequestResponse()
      }
  }

  private fun validateSwoshDTO(dto: SwoshDTO) =
    when {
      dto.amount == null || dto.phone == null || dto.phone.isBlank() ->
        ErrorDTO(reason = "Missing input parameters. 'phone' and 'amount' is required")
      dto.amount < 1 ->
        ErrorDTO(reason = "Minimum allowed amount is 1. Got ${dto.amount}")
      dto.message != null && dto.message.length > 50 ->
        ErrorDTO(reason = "Description is too long. Max 50 chars. Got ${dto.message.length}")
      else -> validatePhoneNumber(dto.phone)
    }

  private fun validatePhoneNumber(phone: String): ErrorDTO? {
    // Seems to be a valid swish number.
    if (phone.startsWith("123") && phone.length == 10) return null

    val phoneUtil = PhoneNumberUtil.getInstance()
    val parsedNumber: Phonenumber.PhoneNumber?
    try {
      parsedNumber = phoneUtil.parse(phone, "SE")
    } catch (e: NumberParseException) {
      return ErrorDTO(reason = "'$phone' is not a valid phone number")
    }
    if (phoneUtil.getNumberType(parsedNumber) != PhoneNumberUtil.PhoneNumberType.MOBILE) {
      return ErrorDTO(reason = "'$phone' is not a mobile number")
    }

    return null
  }

  private fun constructAndInsertNewSwosh(dto: SwoshDTO): Mono<Swosh> {
    return repo.save(dto.toSwosh())
  }

}
