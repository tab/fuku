package com.fuku.plugin.settings

import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.Service
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage

@Service(Service.Level.APP)
@State(
  name = "com.fuku.plugin.settings.Settings",
  storages = [Storage("fuku.xml")],
)
class Settings : PersistentStateComponent<Settings.State> {
  data class State(
    var host: String = "127.0.0.1",
    var port: Int = 9876,
    var token: String = "",
    var pollInterval: Int = 2000,
    var fukuBinaryPath: String = "fuku",
  )

  private var state = State()

  override fun getState(): State = state

  override fun loadState(state: State) {
    this.state = state
  }

  var host: String
    get() = state.host
    set(value) {
      state.host = value
    }

  var port: Int
    get() = state.port
    set(value) {
      state.port = value
    }

  var token: String
    get() = state.token
    set(value) {
      state.token = value
    }

  var pollInterval: Int
    get() = state.pollInterval
    set(value) {
      state.pollInterval = value
    }

  var fukuBinaryPath: String
    get() = state.fukuBinaryPath
    set(value) {
      state.fukuBinaryPath = value
    }

  companion object {
    fun getInstance(): Settings = ApplicationManager.getApplication().getService(Settings::class.java)
  }
}
