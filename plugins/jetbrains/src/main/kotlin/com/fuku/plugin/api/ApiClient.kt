package com.fuku.plugin.api

import kotlinx.serialization.json.Json
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration

class ApiClient(
  private val host: String = "127.0.0.1",
  private val port: Int = 9876,
  private val token: String = "",
  connectTimeout: Duration = Duration.ofSeconds(5),
  private val readTimeout: Duration = Duration.ofSeconds(10),
) {
  private val baseUrl = "http://$host:$port/api/v1"

  private val json =
    Json {
      ignoreUnknownKeys = true
      isLenient = true
    }

  private val client: HttpClient =
    HttpClient.newBuilder()
      .connectTimeout(connectTimeout)
      .build()

  fun getLive(): Result<Probe> = get("/live", authenticated = false)

  fun getReady(): Result<Probe> = get("/ready", authenticated = false)

  fun getStatus(): Result<Status> = get("/status")

  fun listServices(): Result<ServiceList> = get("/services")

  fun getService(id: String): Result<Service> = get("/services/$id")

  fun startService(id: String): Result<ServiceActionAccepted> = post("/services/$id/start")

  fun stopService(id: String): Result<ServiceActionAccepted> = post("/services/$id/stop")

  fun restartService(id: String): Result<ServiceActionAccepted> = post("/services/$id/restart")

  private inline fun <reified T> get(
    path: String,
    authenticated: Boolean = true,
  ): Result<T> {
    val builder =
      HttpRequest.newBuilder()
        .uri(URI.create("$baseUrl$path"))
        .timeout(readTimeout)
        .GET()

    if (authenticated && token.isNotEmpty()) {
      builder.header("Authorization", "Bearer $token")
    }

    return execute(builder.build())
  }

  private inline fun <reified T> post(path: String): Result<T> {
    val builder =
      HttpRequest.newBuilder()
        .uri(URI.create("$baseUrl$path"))
        .timeout(readTimeout)
        .POST(HttpRequest.BodyPublishers.noBody())

    if (token.isNotEmpty()) {
      builder.header("Authorization", "Bearer $token")
    }

    return execute(builder.build())
  }

  private inline fun <reified T> execute(request: HttpRequest): Result<T> {
    return try {
      val response = client.send(request, HttpResponse.BodyHandlers.ofString())
      val body = response.body() ?: ""

      when (response.statusCode()) {
        in 200..299 -> {
          val parsed = json.decodeFromString<T>(body)
          Result.success(parsed)
        }
        else -> {
          val error =
            try {
              json.decodeFromString<ApiError>(body)
            } catch (_: Exception) {
              ApiError(error = "HTTP ${response.statusCode()}: $body")
            }
          Result.failure(ApiException(response.statusCode(), error.error))
        }
      }
    } catch (e: ApiException) {
      Result.failure(e)
    } catch (e: Exception) {
      Result.failure(ApiException(0, e.message ?: "unknown error", e))
    }
  }
}

class ApiException(
  val statusCode: Int,
  override val message: String,
  cause: Throwable? = null,
) : Exception(message, cause)
