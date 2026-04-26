package com.fuku.plugin.toolwindow

import com.fuku.plugin.api.Phase
import com.fuku.plugin.api.ServiceStatus
import com.intellij.ui.JBColor
import com.intellij.ui.table.JBTable
import java.awt.Color
import java.awt.Component
import java.awt.Dimension
import java.awt.Font
import java.awt.Graphics
import java.awt.Graphics2D
import java.awt.RenderingHints
import javax.swing.JComponent
import javax.swing.ListSelectionModel
import javax.swing.SwingConstants
import javax.swing.border.EmptyBorder
import javax.swing.table.DefaultTableCellRenderer
import javax.swing.table.TableCellRenderer

object ServiceColors {
  val running: Color = JBColor(Color(22, 163, 74), Color(74, 222, 128))
  val warning: Color = JBColor(Color(217, 119, 6), Color(251, 191, 36))
  val error: Color = JBColor(Color(220, 38, 38), Color(248, 113, 113))
  val stopped: Color = JBColor(Color(163, 163, 163), Color(128, 128, 128))
  val tierHeader: Color = JBColor(Color(125, 86, 244), Color(141, 107, 255))
  val muted: Color = JBColor(Color(160, 160, 160), Color(110, 110, 110))
  val empty: Color = JBColor(Color(184, 184, 184), Color(74, 74, 74))

  fun forStatus(status: ServiceStatus): Color =
    when (status) {
      ServiceStatus.running -> running
      ServiceStatus.starting, ServiceStatus.restarting -> warning
      ServiceStatus.stopping -> error
      ServiceStatus.failed -> error
      ServiceStatus.stopped -> stopped
    }

  fun forPhase(phase: Phase): Color =
    when (phase) {
      Phase.running -> running
      Phase.startup -> warning
      Phase.stopping -> error
      Phase.stopped -> stopped
    }

  fun forSlot(slot: TimelineSlot): Color =
    when (slot) {
      TimelineSlot.RUNNING -> running
      TimelineSlot.STARTING -> warning
      TimelineSlot.FAILED -> error
      TimelineSlot.STOPPED -> stopped
      TimelineSlot.EMPTY -> empty
    }
}

class ServiceTable(private val serviceModel: ServiceTableModel) : JBTable(serviceModel) {
  var blinkVisible = true
    private set

  private val blinkTimer =
    javax.swing.Timer(500) {
      blinkVisible = !blinkVisible
      repaint()
    }.apply { start() }

  init {
    setShowGrid(false)
    rowHeight = 28
    selectionModel.selectionMode = ListSelectionModel.SINGLE_SELECTION
    autoResizeMode = AUTO_RESIZE_SUBSEQUENT_COLUMNS
    intercellSpacing = Dimension(0, 0)
    putClientProperty("Table.isStriped", false)
    setExpandableItemsEnabled(false)

    columnModel.getColumn(ServiceTableModel.COL_NAME).cellRenderer = NameCellRenderer()
    columnModel.getColumn(ServiceTableModel.COL_STATUS).cellRenderer = StatusCellRenderer()

    val metric = MetricCellRenderer()
    columnModel.getColumn(ServiceTableModel.COL_CPU).cellRenderer = metric
    columnModel.getColumn(ServiceTableModel.COL_MEMORY).cellRenderer = metric
    columnModel.getColumn(ServiceTableModel.COL_PID).cellRenderer = metric
    columnModel.getColumn(ServiceTableModel.COL_UPTIME).cellRenderer = metric

    tableHeader.defaultRenderer = HeaderCellRenderer()
    tableHeader.reorderingAllowed = false

    columnModel.getColumn(ServiceTableModel.COL_NAME).preferredWidth = 180
    columnModel.getColumn(ServiceTableModel.COL_STATUS).preferredWidth = 260
    columnModel.getColumn(ServiceTableModel.COL_CPU).preferredWidth = 70
    columnModel.getColumn(ServiceTableModel.COL_MEMORY).preferredWidth = 70
    columnModel.getColumn(ServiceTableModel.COL_PID).preferredWidth = 70
    columnModel.getColumn(ServiceTableModel.COL_UPTIME).preferredWidth = 70
  }

  override fun prepareRenderer(
    renderer: TableCellRenderer,
    row: Int,
    column: Int,
  ): Component {
    val component = super.prepareRenderer(renderer, row, column)
    if (!isRowSelected(row)) {
      component.background = background
    }
    return component
  }

  override fun changeSelection(
    rowIndex: Int,
    columnIndex: Int,
    toggle: Boolean,
    extend: Boolean,
  ) {
    if (serviceModel.isTierHeader(rowIndex)) {
      val current = selectedRow
      val direction = if (rowIndex >= current || current < 0) 1 else -1
      var target = rowIndex + direction
      while (target in 0 until rowCount && serviceModel.isTierHeader(target)) {
        target += direction
      }
      if (target in 0 until rowCount && !serviceModel.isTierHeader(target)) {
        super.changeSelection(target, columnIndex, toggle, extend)
      }
      return
    }
    super.changeSelection(rowIndex, columnIndex, toggle, extend)
  }

  override fun isCellEditable(
    row: Int,
    column: Int,
  ): Boolean = false

  fun dispose() {
    blinkTimer.stop()
  }
}

// -------------------------------------------------------------------
// Name column: colored status dot + service name, or bold tier header
// -------------------------------------------------------------------

private class NameCellRenderer : JComponent(), TableCellRenderer {
  private var dotColor: Color? = null
  private var dotVisible: Boolean = true
  private var text: String = ""
  private var textColor: Color = Color.WHITE
  private var textFont: Font = Font("Dialog", Font.PLAIN, 12)
  private var bg: Color = Color.BLACK

  init {
    isOpaque = true
  }

  override fun getTableCellRendererComponent(
    table: javax.swing.JTable,
    value: Any?,
    isSelected: Boolean,
    hasFocus: Boolean,
    row: Int,
    column: Int,
  ): Component {
    val model = table.model as ServiceTableModel
    bg = if (isSelected && !model.isTierHeader(row)) table.selectionBackground else table.background

    when (val r = model.getRowType(row)) {
      is TableRow.TierHeader -> {
        dotColor = null
        text = r.tier
        textColor = ServiceColors.tierHeader
        textFont = table.font.deriveFont(Font.BOLD)
      }
      is TableRow.ServiceRow -> {
        val status = r.service.status
        val isBlinking =
          status == ServiceStatus.starting ||
            status == ServiceStatus.stopping ||
            status == ServiceStatus.restarting
        dotColor = if (isSelected) table.selectionForeground else ServiceColors.forStatus(status)
        dotVisible = !isBlinking || (table as ServiceTable).blinkVisible
        text = r.service.name
        textColor = if (isSelected) table.selectionForeground else table.foreground
        textFont = table.font
      }
      null -> {
        dotColor = null
        dotVisible = true
        text = ""
        textFont = table.font
      }
    }
    return this
  }

  override fun paintComponent(g: Graphics) {
    val g2 = g as Graphics2D
    g2.setRenderingHint(RenderingHints.KEY_ANTIALIASING, RenderingHints.VALUE_ANTIALIAS_ON)
    g2.setRenderingHint(RenderingHints.KEY_TEXT_ANTIALIASING, RenderingHints.VALUE_TEXT_ANTIALIAS_ON)

    g2.color = bg
    g2.fillRect(0, 0, width, height)

    g2.font = textFont
    val fm = g2.fontMetrics
    val textY = (height + fm.ascent - fm.descent) / 2
    var x = 10

    if (dotColor != null) {
      val dotFont = textFont.deriveFont(textFont.size2D * 0.7f)
      g2.font = dotFont
      val dotFm = g2.fontMetrics
      val dotChar = if (dotVisible) "\u25C9" else "\u25CB"
      g2.color = dotColor!!
      val dotY = (height + dotFm.ascent - dotFm.descent) / 2
      g2.drawString(dotChar, x, dotY)
      x += dotFm.stringWidth("\u25C9") + 4
    }

    g2.font = textFont
    g2.color = textColor
    g2.drawString(text, x, textY)
  }
}

// -------------------------------------------------------------------
// Status column: painted progress bar blocks + colored status text
// -------------------------------------------------------------------

private class StatusCellRenderer : JComponent(), TableCellRenderer {
  private var timeline: Timeline? = null
  private var statusColor: Color = ServiceColors.stopped
  private var statusText: String = ""
  private var bg: Color = Color.BLACK
  private var showBar: Boolean = false

  init {
    isOpaque = true
  }

  override fun getTableCellRendererComponent(
    table: javax.swing.JTable,
    value: Any?,
    isSelected: Boolean,
    hasFocus: Boolean,
    row: Int,
    column: Int,
  ): Component {
    val model = table.model as ServiceTableModel
    bg = if (isSelected && !model.isTierHeader(row)) table.selectionBackground else table.background
    font = table.font

    val service = model.getServiceAt(row)
    if (service != null) {
      timeline = model.getTimelineAt(row)
      statusColor = if (isSelected) table.selectionForeground else ServiceColors.forStatus(service.status)
      statusText = service.status.name
      showBar = true
    } else {
      timeline = null
      statusText = ""
      showBar = false
    }
    return this
  }

  override fun paintComponent(g: Graphics) {
    val g2 = g as Graphics2D
    g2.setRenderingHint(RenderingHints.KEY_ANTIALIASING, RenderingHints.VALUE_ANTIALIAS_ON)
    g2.setRenderingHint(RenderingHints.KEY_TEXT_ANTIALIASING, RenderingHints.VALUE_TEXT_ANTIALIAS_ON)

    g2.color = bg
    g2.fillRect(0, 0, width, height)
    if (!showBar) return

    val blockWidth = 5
    val blockHeight = 12
    val blockGap = 2
    val blockRadius = 2
    val numBlocks = 20
    val barY = (height - blockHeight) / 2
    var x = 4

    val slots = timeline?.display(numBlocks) ?: List(numBlocks) { TimelineSlot.EMPTY }
    for (slot in slots) {
      g2.color = ServiceColors.forSlot(slot)
      g2.fillRoundRect(x, barY, blockWidth, blockHeight, blockRadius, blockRadius)
      x += blockWidth + blockGap
    }

    x += 8
    g2.color = statusColor
    g2.font = font
    val fm = g2.fontMetrics
    val textY = (height + fm.ascent - fm.descent) / 2
    g2.drawString(statusText, x, textY)
  }
}

// -------------------------------------------------------------------
// Metric columns: right-aligned, muted for empty values
// -------------------------------------------------------------------

private class MetricCellRenderer : DefaultTableCellRenderer() {
  override fun getTableCellRendererComponent(
    table: javax.swing.JTable,
    value: Any?,
    isSelected: Boolean,
    hasFocus: Boolean,
    row: Int,
    column: Int,
  ): Component {
    super.getTableCellRendererComponent(table, value, isSelected, hasFocus, row, column)

    val model = table.model as ServiceTableModel
    if (model.isTierHeader(row)) {
      text = ""
      background = table.background
      horizontalAlignment = SwingConstants.RIGHT
      border = EmptyBorder(0, 4, 0, 12)
      return this
    }

    val error = model.getServiceError(row)
    if (error != null) {
      if (column == ServiceTableModel.COL_CPU) {
        horizontalAlignment = SwingConstants.LEFT
        border = EmptyBorder(0, 8, 0, 4)
        if (!isSelected) foreground = ServiceColors.error
      } else {
        text = ""
        horizontalAlignment = SwingConstants.RIGHT
        border = EmptyBorder(0, 4, 0, 12)
      }
      return this
    }

    horizontalAlignment = SwingConstants.RIGHT
    border = EmptyBorder(0, 4, 0, 12)
    if (!isSelected) {
      val v = value?.toString() ?: ""
      foreground = if (v.isEmpty()) ServiceColors.muted else table.foreground
    }
    return this
  }
}

// -------------------------------------------------------------------
// Column headers: muted text, alignment matching data columns
// -------------------------------------------------------------------

private class HeaderCellRenderer : DefaultTableCellRenderer() {
  override fun getTableCellRendererComponent(
    table: javax.swing.JTable,
    value: Any?,
    isSelected: Boolean,
    hasFocus: Boolean,
    row: Int,
    column: Int,
  ): Component {
    super.getTableCellRendererComponent(table, value, isSelected, hasFocus, row, column)
    foreground = ServiceColors.muted
    background = table.background
    font = table.font
    border = EmptyBorder(4, 4, 6, 12)

    horizontalAlignment =
      when (column) {
        ServiceTableModel.COL_NAME, ServiceTableModel.COL_STATUS -> SwingConstants.LEFT
        else -> SwingConstants.RIGHT
      }
    return this
  }
}
