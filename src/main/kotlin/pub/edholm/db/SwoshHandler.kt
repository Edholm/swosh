package pub.edholm.db

import org.springframework.http.HttpStatus
import org.springframework.http.MediaType
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.server.ServerRequest
import org.springframework.web.reactive.function.server.ServerResponse
import org.springframework.web.reactive.function.server.ServerResponse.*
import org.springframework.web.reactive.function.server.body
import pub.edholm.badRequestResponse
import pub.edholm.domain.*
import reactor.core.publisher.Mono
import reactor.core.publisher.toMono
import java.net.URI

@Component
class SwoshHandler(private val repo: SwoshRepository) {
    fun renderIndex(req: ServerRequest) = ok().render("index")

    fun redirectToSwish(req: ServerRequest) =
            repo.findOne(req.pathVariable("id"))
                    .then { s ->
                        temporaryRedirect(s.toSwishDataDTO().generateUri())
                                .build()
                    }
                    .otherwiseIfEmpty(temporaryRedirect(URI.create("/")).build())

    fun createSwosh(req: ServerRequest): Mono<ServerResponse> {
        return req.bodyToMono(SwoshDTO::class.java)
                .then { dto ->
                    val swoshErrorDTO = validateSwoshDTO(dto)
                    when {
                        swoshErrorDTO != null -> swoshErrorDTO.badRequestResponse()
                        else ->
                            constructAndInsertNewSwosh(dto)
                                    .then { (id) ->
                                        ok()
                                                .contentType(MediaType.APPLICATION_JSON_UTF8)
                                                .body(SwoshUrlDTO(id).toMono())
                                    }
                                    .otherwise {
                                        status(HttpStatus.INTERNAL_SERVER_ERROR)
                                                .contentType(MediaType.APPLICATION_JSON_UTF8)
                                                .body(ErrorDTO(reason = "Unable to generate Swosh!").toMono())
                                    }
                    }
                }
                .otherwise { _ ->
                    ErrorDTO(reason = "Invalid input format!")
                            .badRequestResponse()
                }
    }

    private fun validateSwoshDTO(dto: SwoshDTO) =
            when {
                dto.amount == null || dto.phone == null || dto.phone.isBlank() ->
                    ErrorDTO(reason = "Missing input parameters. 'phone' and 'amount' is required")
                !dto.phone.matches(Regex("[0-9 +-]{10,15}")) ->
                    ErrorDTO(reason = "Invalid phone number. Got: ${dto.phone}")
                dto.amount < 1 ->
                    ErrorDTO(reason = "Minimum allowed amount is 1. Got ${dto.amount}")
                dto.message != null && dto.message.length > 50 ->
                    ErrorDTO(reason = "Description is too long. Max 50 chars. Got ${dto.message.length}")
                else -> null
            }

    private fun constructAndInsertNewSwosh(dto: SwoshDTO): Mono<Swosh> {
        return repo.save(dto.toSwosh())
    }
}
