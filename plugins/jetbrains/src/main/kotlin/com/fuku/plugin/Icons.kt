package com.fuku.plugin

import com.intellij.openapi.util.IconLoader
import javax.swing.Icon

object Icons {
  @JvmField
  val ToolWindow: Icon = IconLoader.getIcon("/icons/fuku.svg", Icons::class.java)

  @JvmField
  val Start: Icon = IconLoader.getIcon("/expui/debugger/threadRunning.svg", Icons::class.java)

  @JvmField
  val Stop: Icon = IconLoader.getIcon("/expui/run/stop.svg", Icons::class.java)

  @JvmField
  val Restart: Icon = IconLoader.getIcon("/expui/run/restart.svg", Icons::class.java)

  @JvmField
  val RunAll: Icon = IconLoader.getIcon("/expui/actions/runAll.svg", Icons::class.java)

  @JvmField
  val Settings: Icon = IconLoader.getIcon("/expui/general/settings.svg", Icons::class.java)
}
