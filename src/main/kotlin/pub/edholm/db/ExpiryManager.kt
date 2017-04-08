package pub.edholm.db

import org.springframework.scheduling.annotation.Scheduled
import org.springframework.stereotype.Component
import java.time.Instant

@Component
class ExpiryManager(private val swoshRepo: SwoshRepository) {
    @Scheduled(fixedDelay = 60000) // TODO: use application.properties instead
    fun expireOldLinks() {
        swoshRepo.delete(
                swoshRepo.findByExpiresOnBefore(Instant.now()))
                .subscribe()
    }
}