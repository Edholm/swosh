package pub.edholm

import com.samskivert.mustache.Mustache
import org.springframework.boot.SpringApplication
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.autoconfigure.mustache.MustacheProperties
import org.springframework.boot.autoconfigure.mustache.MustacheResourceTemplateLoader
import org.springframework.boot.web.reactive.result.view.MustacheViewResolver
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.scheduling.annotation.EnableScheduling
import org.springframework.web.reactive.config.WebFluxConfigurer


@SpringBootApplication
@EnableScheduling
@Configuration
class SwoshApplication(private val props: MustacheProperties) : WebFluxConfigurer {

    private val mustacheCompiler = Mustache
            .compiler()
            .escapeHTML(false)
            .withLoader(MustacheResourceTemplateLoader(props.prefix, props.suffix))

    @Bean
    fun viewResolver() = MustacheViewResolver(mustacheCompiler).apply {
        setPrefix(props.prefix)
        setSuffix(props.suffix)
    }
}

fun main(args: Array<String>) {
    SpringApplication.run(SwoshApplication::class.java, *args)
}

