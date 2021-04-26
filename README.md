# Swosh.me ![](https://github.com/Edholm/swosh/workflows/Build,%20test,%20and%20deploy/badge.svg)

Used for creating short links that you can send to your friends, thus allowing them to easily launch Swish with your preset information.

This is useful since https:// links are almost universally clickable.

## Requirements
* Gradle
* Java 15
* _Kotlin_ 
* _spring-boot_

## API
* `POST /api/create`
Accepts `application/json` and the body is as follows:
```json
	{
		"phone": "07xxxxxx",
		"amount": 100,
		"message": "example message",
		"expireAfterSeconds": null, // (optional, 0 or null is never expire)
	}
```

## Deploy
You can "deploy" Swosh using `docker-compose`. See `deploy.sh` for an example of how it can be done.

## License

See the [LICENSE](LICENSE.md) file for license rights and limitations (MIT).
