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

import com.samskivert.mustache.Mustache
import com.samskivert.mustache.Template
import org.springframework.core.io.Resource
import org.springframework.web.reactive.result.view.UrlBasedViewResolver
import org.springframework.web.reactive.result.view.View
import org.springframework.web.reactive.result.view.ViewResolver
import reactor.core.publisher.Mono
import java.io.IOException
import java.io.InputStreamReader
import java.nio.charset.Charset
import java.nio.charset.StandardCharsets
import java.util.*

/**
 * Spring WebFlux [ViewResolver] for Mustache.

 * @author Dave Syer
 * *
 * @author Andy Wilkinson
 * *
 * @author Phillip Webb
 * *
 * @author Sebastien Deleuze
 */
class MustacheViewResolver : UrlBasedViewResolver() {

    private var compiler: Mustache.Compiler = Mustache.compiler().escapeHTML(false)

    private var charset = StandardCharsets.UTF_8

    init {
        viewClass = requiredViewClass()
    }

    override fun requiredViewClass(): Class<*> {
        return MustacheView::class.java
    }

    /**
     * Set the compiler.

     * @param compiler the compiler
     */
    fun setCompiler(compiler: Mustache.Compiler) {
        this.compiler = compiler
    }

    /**
     * Set the charset.

     * @param charset the charset
     */
    fun setCharset(charset: Charset) {
        this.charset = charset
    }


    override fun resolveViewName(viewName: String, locale: Locale): Mono<View> {
        val resource = applicationContext.getResource(prefix + viewName + suffix)
        return super.resolveViewName(viewName, locale).map { view ->
            val mustacheView = view as MustacheView
            mustacheView.setTemplate(createTemplate(resource))
            view
        }
    }

    private fun createTemplate(resource: Resource): Template {

        try {
            InputStreamReader(resource.inputStream, this.charset).use { reader -> return this.compiler.compile(reader) }
        } catch (ioe: IOException) {
            throw IllegalStateException(ioe)
        }

    }
}