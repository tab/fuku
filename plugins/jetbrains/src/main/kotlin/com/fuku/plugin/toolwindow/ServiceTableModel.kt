package com.fuku.plugin.toolwindow

import com.fuku.plugin.api.Service
import com.fuku.plugin.api.ServiceStatus
import javax.swing.table.AbstractTableModel

sealed class TableRow {
  data class TierHeader(val tier: String) : TableRow()

  data class ServiceRow(val service: Service) : TableRow()
}

enum class TimelineSlot {
  EMPTY,
  RUNNING,
  STARTING,
  FAILED,
  STOPPED,
  ;

  companion object {
    fun forStatus(status: ServiceStatus): TimelineSlot =
      when (status) {
        ServiceStatus.running -> RUNNING
        ServiceStatus.starting, ServiceStatus.restarting, ServiceStatus.stopping -> STARTING
        ServiceStatus.failed -> FAILED
        ServiceStatus.stopped -> STOPPED
      }
  }
}

class Timeline(private val capacity: Int = 20) {
  private val data = Array(capacity) { TimelineSlot.EMPTY }
  private var head = 0
  private var count = 0

  fun append(slot: TimelineSlot) {
    data[head] = slot
    head = (head + 1) % capacity
    if (count < capacity) count++
  }

  fun display(width: Int): List<TimelineSlot> {
    val observed = chronological()
    if (observed.size >= width) return observed.takeLast(width)
    return observed + List(width - observed.size) { TimelineSlot.EMPTY }
  }

  private fun chronological(): List<TimelineSlot> {
    if (count == 0) return emptyList()
    if (count < capacity) return data.take(count).toList()
    return (0 until capacity).map { data[(head + it) % capacity] }
  }
}

class ServiceTableModel : AbstractTableModel() {
  private var allServices: List<Service> = emptyList()
  private var rows: List<TableRow> = emptyList()
  private var filterQuery: String = ""
  private val timelines = mutableMapOf<String, Timeline>()

  val isFiltered: Boolean get() = filterQuery.isNotEmpty()

  fun updateServices(services: List<Service>) {
    allServices = services
    sampleTimelines(services)
    rebuildRows()
  }

  fun getTimelineAt(row: Int): Timeline? {
    val service = getServiceAt(row) ?: return null
    return timelines[service.id]
  }

  private fun sampleTimelines(services: List<Service>) {
    for (svc in services) {
      val timeline = timelines.getOrPut(svc.id) { Timeline() }
      timeline.append(TimelineSlot.forStatus(svc.status))
    }
    val currentIds = services.map { it.id }.toSet()
    timelines.keys.removeAll { it !in currentIds }
  }

  fun setFilter(query: String) {
    filterQuery = query.lowercase()
    rebuildRows()
  }

  fun clearFilter() {
    filterQuery = ""
    rebuildRows()
  }

  private fun rebuildRows() {
    val filtered =
      if (filterQuery.isEmpty()) {
        allServices
      } else {
        allServices.filter { it.name.lowercase().contains(filterQuery) }
      }
    rows = buildRows(filtered)
    fireTableDataChanged()
  }

  fun getRowType(row: Int): TableRow? = rows.getOrNull(row)

  fun getServiceAt(row: Int): Service? {
    return when (val r = rows.getOrNull(row)) {
      is TableRow.ServiceRow -> r.service
      else -> null
    }
  }

  fun isTierHeader(row: Int): Boolean = rows.getOrNull(row) is TableRow.TierHeader

  fun findRowForServiceId(id: String): Int = rows.indexOfFirst { it is TableRow.ServiceRow && it.service.id == id }

  fun getServiceError(row: Int): String? = getServiceAt(row)?.error

  override fun getRowCount(): Int = rows.size

  override fun getColumnCount(): Int = COLUMNS.size

  override fun getColumnName(column: Int): String = COLUMNS[column]

  override fun getColumnClass(column: Int): Class<*> = String::class.java

  override fun getValueAt(
    rowIndex: Int,
    columnIndex: Int,
  ): Any {
    return when (val row = rows[rowIndex]) {
      is TableRow.TierHeader -> if (columnIndex == COL_NAME) row.tier else ""
      is TableRow.ServiceRow -> {
        val svc = row.service
        val hasError = svc.error != null
        val active = svc.status == ServiceStatus.running && svc.pid > 0
        when (columnIndex) {
          COL_NAME -> svc.name
          COL_STATUS -> svc.status.name
          COL_CPU ->
            if (hasError) {
              svc.error!!
            } else if (active) {
              "%.1f%%".format(svc.cpu)
            } else {
              ""
            }
          COL_MEMORY ->
            if (hasError) {
              ""
            } else if (active) {
              formatMemory(svc.memory)
            } else {
              ""
            }
          COL_PID ->
            if (hasError) {
              ""
            } else if (svc.pid > 0) {
              svc.pid.toString()
            } else {
              ""
            }
          COL_UPTIME ->
            if (hasError) {
              ""
            } else if (active) {
              formatUptime(svc.uptime)
            } else {
              ""
            }
          else -> ""
        }
      }
    }
  }

  private fun buildRows(services: List<Service>): List<TableRow> {
    if (services.isEmpty()) return emptyList()

    val result = mutableListOf<TableRow>()
    val grouped = services.groupBy { it.tier }
    for ((tier, tierServices) in grouped) {
      result.add(TableRow.TierHeader(tier))
      tierServices.forEach { result.add(TableRow.ServiceRow(it)) }
    }
    return result
  }

  companion object {
    const val COL_NAME = 0
    const val COL_STATUS = 1
    const val COL_CPU = 2
    const val COL_MEMORY = 3
    const val COL_PID = 4
    const val COL_UPTIME = 5

    val COLUMNS = arrayOf("", "status", "cpu", "mem", "pid", "uptime")

    fun formatMemory(bytes: Long): String {
      if (bytes <= 0) return ""
      val mb = bytes / (1024.0 * 1024.0)
      if (mb >= 1024.0) {
        return "%.1fGB".format(mb / 1024.0)
      }
      return "%.0fMB".format(mb)
    }

    fun formatUptime(seconds: Long): String {
      if (seconds <= 0) return ""
      val hours = seconds / 3600
      val minutes = (seconds % 3600) / 60
      val secs = seconds % 60
      return when {
        hours > 0 -> "%d:%02d:%02d".format(hours, minutes, secs)
        else -> "%02d:%02d".format(minutes, secs)
      }
    }
  }
}
