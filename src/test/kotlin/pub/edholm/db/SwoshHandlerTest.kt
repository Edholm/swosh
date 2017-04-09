package pub.edholm.db

import org.assertj.core.api.Assertions.assertThat
import org.junit.Before
import org.junit.Test
import org.mockito.Mock
import org.mockito.Mockito
import org.springframework.test.web.reactive.server.WebTestClient
import pub.edholm.domain.*
import pub.edholm.web.Router
import reactor.core.publisher.Mono
import reactor.core.publisher.toMono

class SwoshHandlerTest {

    @Mock
    private lateinit var swoshRepo: SwoshRepository

    private lateinit var swoshHandler: SwoshHandler

    private lateinit var router: Router

    private lateinit var webTestClient: WebTestClient

    @Before
    fun setUp() {
        swoshRepo = Mockito.mock(SwoshRepository::class.java)
        swoshHandler = SwoshHandler(swoshRepo)
        router = Router(swoshHandler)
        webTestClient = WebTestClient.bindToRouterFunction(router.route()).build()
    }

    @Test
    fun `Redirect to Swish when id does not exist`() {
        Mockito.`when`(swoshRepo.findOne("asdfg")).thenReturn(Mono.empty())
        webTestClient.get()
                .uri("/asdfg")
                .exchange()
                .expectStatus().is3xxRedirection
                .expectHeader().valueEquals("location", "/")
                .expectBody().isEmpty
    }

    @Test
    fun `Redirect to Swish`() {
        val swoshId = "asdf123"
        Mockito.`when`(swoshRepo.findOne(swoshId)).thenReturn(Mono.just(Swosh(swoshId)))
        webTestClient.get()
                .uri("/$swoshId/redir")
                .exchange()
                .expectStatus().is3xxRedirection
                .expectHeader().valueMatches("location", "swish://payment\\?data=.+")
                .expectBody().isEmpty
    }

    @Test
    fun `Create Swosh with invalid body`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwishDataDTO(1, StringValue("herp"), IntValue(1), StringValue("derp"))) // Just a data class with "wrong" body
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value().isEqualTo(ErrorDTO(error = true, reason = "Invalid input format!"))
    }

    @Test
    fun `Create Swosh with missing amount`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", null, "msg", 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Missing input parameters. 'phone' and 'amount' is required"))
    }

    @Test
    fun `Create Swosh with missing phone number`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO(null, 100, "msg", 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Missing input parameters. 'phone' and 'amount' is required"))
    }

    @Test
    fun `Create Swosh with blank phone`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("          ", 100, "msg", 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Missing input parameters. 'phone' and 'amount' is required"))
    }

    @Test
    fun `Create Swosh with zero amount`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", 0, "msg", 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Minimum allowed amount is 1. Got 0"))
    }

    @Test
    fun `Create Swosh with negative amount`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", -1, "msg", 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Minimum allowed amount is 1. Got -1"))
    }

    @Test
    fun `Create Swosh with too long description`() {
        val longMsg = "a" * 51
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", 100, longMsg, 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Description is too long. Max 50 chars. Got 51"))
    }

    @Test
    fun `Create Swosh with max length description`() {
        Mockito.`when`(swoshRepo.save(Mockito.any(Swosh::class.java)))
                .thenReturn(Swosh(id = "edaeda1").toMono())

        val longMsg = "b" * 50
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", 100, longMsg, 100))
                .exchange()
                .expectStatus().isOk
                .expectBody(SwoshUrlDTO::class.java).value()
                .isEqualTo(SwoshUrlDTO("edaeda1"))
    }


    @Test
    fun `Create Swosh with invalid phone number`() {
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("070000000a", 100, "msg", 100))
                .exchange()
                .expectStatus().isBadRequest
                .expectBody(ErrorDTO::class.java).value()
                .isEqualTo(ErrorDTO(error = true, reason = "Invalid phone number. Got: 070000000a"))
    }

    @Test
    fun `Create Swosh with correct phone number`() {
        Mockito.`when`(swoshRepo.save(Mockito.any(Swosh::class.java)))
                .thenReturn(Swosh(id = "validphone").toMono())

        val validPhoneNumbers = listOf("072-000 00 00",
                "0730 00 00 00", "+46760000000", "0790000000", "070 000 00 00", "1230000000")

        validPhoneNumbers.forEach { phone ->
            webTestClient.post()
                    .uri("/api/create")
                    .body(SwoshDTO(phone, 100, "msg", 100))
                    .exchange()
                    .expectStatus().isOk
                    .expectBody(SwoshUrlDTO::class.java).value()
                    .isEqualTo(SwoshUrlDTO("validphone"))
        }
    }

    @Test
    fun `Create Swosh with missing message`() {
        Mockito.`when`(swoshRepo.save(Mockito.any(Swosh::class.java)))
                .thenReturn(Swosh(id = "nomsg").toMono())
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", 100, null, 100))
                .exchange()
                .expectStatus().isOk
                .expectBody(SwoshUrlDTO::class.java).value()
                .isEqualTo(SwoshUrlDTO("nomsg"))
    }

    @Test
    fun `Create Swosh with missing expiry time`() {
        Mockito.`when`(swoshRepo.save(Mockito.any(Swosh::class.java)))
                .thenReturn(Swosh(id = "noexpire").toMono())
        webTestClient.post()
                .uri("/api/create")
                .body(SwoshDTO("0700000000", 100, "msg", null))
                .exchange()
                .expectStatus().isOk
                .expectBody(SwoshUrlDTO::class.java).value()
                .isEqualTo(SwoshUrlDTO("noexpire"))
    }

    @Test
    fun `toSwosh extension with expiry never`() {
        val swoshExpire0 = SwoshDTO("0700000000", 100, "msg", 0)
        val swoshExpireNull = SwoshDTO("0700000000", 100, "msg", null)

        assertThat(swoshExpire0.toSwosh())
                .extracting("payee", "amount", "description", "expiresOn")
                .containsExactly("0700000000", 100, "msg", null)

        assertThat(swoshExpireNull.toSwosh())
                .extracting("payee", "amount", "description", "expiresOn")
                .containsExactly("0700000000", 100, "msg", null)

    }

    @Test
    fun toSwoshExtension() {
        val swoshDTO = SwoshDTO(" 070-000 00 00 ", 100, "msg", 0)
        assertThat(swoshDTO.toSwosh())
                .extracting("payee", "amount", "description", "expiresOn")
                .containsExactly("0700000000", 100, "msg", null)
    }

    private operator fun String.times(i: Int): String {
        var result = ""
        for (j in 0 until i) {
            result += this
        }
        return result
    }
}
