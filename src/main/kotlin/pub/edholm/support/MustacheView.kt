/*
 * Copyright 2002-2016 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pub.edholm.support

import com.samskivert.mustache.Template
import org.springframework.http.HttpStatus.INTERNAL_SERVER_ERROR
import org.springframework.http.MediaType
import org.springframework.web.reactive.result.view.AbstractUrlBasedView
import org.springframework.web.reactive.result.view.View
import org.springframework.web.server.ResponseStatusException
import org.springframework.web.server.ServerWebExchange
import reactor.core.publisher.Flux
import reactor.core.publisher.Mono
import java.io.IOException
import java.io.OutputStreamWriter
import java.util.*

/**
 * Spring WebFlux [View] using the Mustache template engine.

 * @author Dave Syer
 * *
 * @author Phillip Webb
 * *
 * @author Sebastien Deleuze
 */
class MustacheView : AbstractUrlBasedView {

    private var template: Template? = null

    constructor() {
        requestContextAttribute = "context"
    }

    constructor(template: Template) {
        this.template = template
    }

    override fun renderInternal(model: Map<String, Any>, contentType: MediaType?, exchange: ServerWebExchange): Mono<Void> {
        if (this.template != null) {

            val dataBuffer = exchange.response.bufferFactory().allocateBuffer()
            val writer = OutputStreamWriter(dataBuffer.asOutputStream(), defaultCharset)
            try {
                this.template!!.execute(model, writer)
            } catch (ex: Exception) {
                val message = "Error while rendering " + url + ": " + ex.message
                logger.error(message)
                return Mono.error<Void>(ResponseStatusException(INTERNAL_SERVER_ERROR, message))
            }

            return exchange.response.writeWith(Flux.just(dataBuffer)).doOnSubscribe { s ->
                try {
                    writer.flush()
                } catch (ex: IOException) {
                    val message = "Could not load Mustache template for URL [$url]"
                    throw IllegalStateException(message, ex)
                }
            }
        }
        return Mono.empty<Void>()
    }

    fun setTemplate(template: Template) {
        this.template = template
    }

    @Throws(Exception::class)
    override fun checkResourceExists(locale: Locale): Boolean {
        return applicationContext.getResource(url).exists()
    }

}