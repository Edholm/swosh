package pub.edholm

import org.springframework.http.MediaType
import org.springframework.web.reactive.function.server.ServerResponse.badRequest
import org.springframework.web.reactive.function.server.body
import pub.edholm.domain.ErrorDTO
import reactor.kotlin.core.publisher.toMono

fun ErrorDTO.badRequestResponse() =
  badRequest()
    .contentType(MediaType.APPLICATION_JSON)
    .body(this.toMono<ErrorDTO>())
