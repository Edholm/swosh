package pub.edholm.services

import org.springframework.scheduling.annotation.Scheduled
import org.springframework.stereotype.Service
import pub.edholm.db.SwoshRepository
import java.time.Instant

@Service
class ExpiryService(private val swoshRepo: SwoshRepository) {

  @Scheduled(fixedRateString = "\${swosh.expire.rate}", initialDelay = 1337)
  fun expireOldLinks() {
    swoshRepo.deleteAll(swoshRepo.findByExpiresOnBefore(Instant.now()))
      .subscribe()
  }
}