package pub.edholm.db

import org.springframework.scheduling.annotation.Scheduled
import org.springframework.stereotype.Component
import java.time.Instant

@Component
class ExpiryManager(private val swoshRepo: SwoshRepository) {
    @Scheduled(fixedRateString = "\${swosh.expire.rate}", initialDelay = 1337)
    fun expireOldLinks() {
        swoshRepo.deleteAll(
                swoshRepo.findByExpiresOnBefore(Instant.now()))
                .subscribe()
    }
}