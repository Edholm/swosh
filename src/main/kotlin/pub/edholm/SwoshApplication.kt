package pub.edholm

import org.springframework.boot.SpringApplication
import org.springframework.boot.autoconfigure.SpringBootApplication

@SpringBootApplication
class SwoshApplication

fun main(args: Array<String>) {
    SpringApplication.run(SwoshApplication::class.java, *args)
}

