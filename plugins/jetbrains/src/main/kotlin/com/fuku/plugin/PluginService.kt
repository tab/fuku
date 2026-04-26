package com.fuku.plugin

import com.fuku.plugin.api.ApiClient
import com.fuku.plugin.api.ServiceActionAccepted
import com.fuku.plugin.api.Status
import com.fuku.plugin.settings.Settings
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.Service
import com.intellij.openapi.diagnostic.Logger
import java.util.Timer
import java.util.TimerTask
import java.util.concurrent.CopyOnWriteArrayList
import com.fuku.plugin.api.Service as FukuService

data class PluginState(
  val connected: Boolean = false,
  val status: Status? = null,
  val services: List<FukuService> = emptyList(),
)

interface PluginStateListener {
  fun onStateChanged(state: PluginState)
}

@Service(Service.Level.APP)
class PluginService : Disposable {
  private val log = Logger.getInstance(PluginService::class.java)
  private val listeners = CopyOnWriteArrayList<PluginStateListener>()

  @Volatile
  var state: PluginState = PluginState()
    private set

  @Volatile
  private var client: ApiClient = createClient()

  @Volatile
  private var timer: Timer? = null

  @Volatile
  private var lastSettingsSnapshot: SettingsSnapshot = takeSettingsSnapshot()

  init {
    startPolling()
  }

  fun addListener(listener: PluginStateListener) {
    listeners.add(listener)
    listener.onStateChanged(state)
  }

  fun removeListener(listener: PluginStateListener) {
    listeners.remove(listener)
  }

  fun startService(id: String): Result<ServiceActionAccepted> = client.startService(id)

  fun stopService(id: String): Result<ServiceActionAccepted> = client.stopService(id)

  fun restartService(id: String): Result<ServiceActionAccepted> = client.restartService(id)

  fun refresh() {
    poll()
  }

  private fun startPolling() {
    stopPolling()
    val interval = lastSettingsSnapshot.pollInterval.toLong()
    timer =
      Timer("fuku-poller", true).apply {
        scheduleAtFixedRate(
          object : TimerTask() {
            override fun run() {
              poll()
            }
          },
          0, interval,
        )
      }
  }

  private fun stopPolling() {
    timer?.cancel()
    timer = null
  }

  private fun poll() {
    val snapshot = takeSettingsSnapshot()
    if (snapshot != lastSettingsSnapshot) {
      lastSettingsSnapshot = snapshot
      client = createClient()
      startPolling()
      return
    }

    val statusResult = client.getStatus()
    val servicesResult = client.listServices()

    val newState =
      if (statusResult.isSuccess && servicesResult.isSuccess) {
        PluginState(
          connected = true,
          status = statusResult.getOrNull(),
          services = servicesResult.getOrNull()?.services ?: emptyList(),
        )
      } else {
        if (state.connected) {
          log.info("fuku connection lost")
        }
        PluginState(connected = false)
      }

    if (newState != state) {
      state = newState
      notifyListeners(newState)
    }
  }

  private fun notifyListeners(snapshot: PluginState) {
    for (listener in listeners) {
      try {
        listener.onStateChanged(snapshot)
      } catch (e: Exception) {
        log.warn("Listener threw exception", e)
      }
    }
  }

  override fun dispose() {
    stopPolling()
    listeners.clear()
  }

  companion object {
    fun getInstance(): PluginService = ApplicationManager.getApplication().getService(PluginService::class.java)
  }

  private fun createClient(): ApiClient {
    val settings = Settings.getInstance()
    return ApiClient(
      host = settings.host,
      port = settings.port,
      token = settings.token,
    )
  }

  private fun takeSettingsSnapshot(): SettingsSnapshot {
    val settings = Settings.getInstance()
    return SettingsSnapshot(
      host = settings.host,
      port = settings.port,
      token = settings.token,
      pollInterval = settings.pollInterval,
    )
  }

  private data class SettingsSnapshot(
    val host: String,
    val port: Int,
    val token: String,
    val pollInterval: Int,
  )
}
