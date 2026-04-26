@file:Suppress("ktlint:standard:enum-entry-name-case")

package com.fuku.plugin.api

import kotlinx.serialization.Serializable

@Serializable
enum class ServiceStatus {
  starting,
  running,
  stopping,
  restarting,
  stopped,
  failed,
}

@Serializable
enum class ActionStatus {
  starting,
  stopping,
  restarting,
}

@Serializable
enum class Phase {
  startup,
  running,
  stopping,
  stopped,
}

@Serializable
data class Service(
  val id: String,
  val name: String,
  val tier: String,
  val status: ServiceStatus,
  val watching: Boolean,
  val error: String? = null,
  val pid: Int,
  val cpu: Double,
  val memory: Long,
  val uptime: Long,
)

@Serializable
data class ServiceList(
  val services: List<Service>,
)

@Serializable
data class ServiceActionAccepted(
  val id: String,
  val name: String,
  val action: String,
  val status: ActionStatus,
)

@Serializable
data class ServiceCounts(
  val total: Int,
  val starting: Int,
  val running: Int,
  val stopping: Int,
  val restarting: Int,
  val stopped: Int,
  val failed: Int,
)

@Serializable
data class Status(
  val version: String,
  val profile: String,
  val phase: Phase,
  val uptime: Long,
  val services: ServiceCounts,
)

@Serializable
data class Probe(
  val status: String,
)

@Serializable
data class ApiError(
  val error: String,
)
