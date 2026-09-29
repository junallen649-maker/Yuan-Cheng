import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hbb/common.dart';
import 'package:flutter_hbb/models/peer_model.dart';
import 'package:flutter_hbb/models/platform_model.dart';

/// UU 远程风格 - 视图 A：设备详情控制台 (对应参考图 1)
class DeskDeviceDetailView extends StatelessWidget {
  final Peer peer;
  final VoidCallback? onPeerUpdated;

  const DeskDeviceDetailView({
    Key? key,
    required this.peer,
    this.onPeerUpdated,
  }) : super(key: key);

  String get displayName {
    if (peer.alias.isNotEmpty) return peer.alias;
    if (peer.hostname.isNotEmpty) return peer.hostname;
    return peer.id;
  }

  void _onConnect(BuildContext context, {
    bool isFileTransfer = false,
    bool isTerminal = false,
    bool isTcpTunneling = false,
  }) {
    connect(
      context,
      peer.id,
      isFileTransfer: isFileTransfer,
      isTerminal: isTerminal,
      isTcpTunneling: isTcpTunneling,
    );
  }

  void _sendQuickAction(BuildContext context, String actionKey) {
    // 快速启动指令：先发起连接或发送特定快捷动作
    switch (actionKey) {
      case 'lock':
        showToast('正在发送锁定屏幕指令...');
        _onConnect(context);
        break;
      case 'restart':
        showToast('正在向设备发起系统操作...');
        _onConnect(context);
        break;
      case 'taskmgr':
        showToast('正在呼出远端任务管理器...');
        _onConnect(context);
        break;
      case 'cad':
        showToast('正在发送 Ctrl+Alt+Del 组合键...');
        _onConnect(context);
        break;
      default:
        showToast('快捷指令已就绪');
        break;
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final cardBg = isDark ? const Color(0xFF1E293B) : Colors.white;
    final borderColor = isDark ? const Color(0xFF334155) : const Color(0xFFE2E8F0);
    final primaryColor = const Color(0xFF1E6FFF);
    final isOnline = peer.online;

    return TweenAnimationBuilder<double>(
      tween: Tween<double>(begin: 0.0, end: 1.0),
      duration: const Duration(milliseconds: 220),
      curve: Curves.easeOutCubic,
      builder: (context, animVal, child) {
        return Opacity(
          opacity: animVal,
          child: Transform.translate(
            offset: Offset(0, 10 * (1 - animVal)),
            child: child,
          ),
        );
      },
      child: SingleChildScrollView(
        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 顶部状态栏：在线/离线 Badge + 设备名称 + 更多菜单
            Row(
              children: [
                // 状态 Badge
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: isOnline
                        ? (isDark ? const Color(0xFF064E3B) : const Color(0xFFDCFCE7))
                        : (isDark ? const Color(0xFF334155) : const Color(0xFFF1F5F9)),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: 6,
                        height: 6,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: isOnline ? const Color(0xFF10B981) : const Color(0xFF94A3B8),
                        ),
                      ),
                      const SizedBox(width: 6),
                      Text(
                        isOnline ? '在线' : '离线',
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: isOnline ? const Color(0xFF10B981) : const Color(0xFF64748B),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 12),

                // 设备名称
                Expanded(
                  child: Text(
                    displayName,
                    style: const TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.bold,
                      letterSpacing: 0.5,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),

                // 更多菜单
                PopupMenuButton<String>(
                  icon: const Icon(Icons.more_vert, color: Colors.grey),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                  onSelected: (val) {
                    if (val == 'copy_id') {
                      Clipboard.setData(ClipboardData(text: peer.id));
                      showToast('设备ID已复制: ${peer.id}');
                    } else if (val == 'delete') {
                      bind.mainRemovePeer(id: peer.id);
                      showToast('已从设备列表中移除');
                      onPeerUpdated?.call();
                    }
                  },
                  itemBuilder: (context) => [
                    PopupMenuItem(
                      value: 'copy_id',
                      child: Row(
                        children: [
                          const Icon(Icons.copy_outlined, size: 18),
                          const SizedBox(width: 10),
                          Text('复制设备ID (${peer.id})'),
                        ],
                      ),
                    ),
                    const PopupMenuItem(
                      value: 'delete',
                      child: Row(
                        children: [
                          Icon(Icons.delete_outline, size: 18, color: Colors.red),
                          SizedBox(width: 10),
                          Text('移除此设备', style: TextStyle(color: Colors.red)),
                        ],
                      ),
                    ),
                  ],
                ),
              ],
            ),
            const SizedBox(height: 18),

            // 主控预览大卡片 (16:9 比例 Win11 几何壁纸质感，带悬浮微光与微动效)
            _InteractivePreviewCard(
              cardBg: cardBg,
              borderColor: borderColor,
              isDark: isDark,
              onTap: () => _onConnect(context),
              toolbar: Container(
                padding: const EdgeInsets.symmetric(vertical: 12),
                decoration: BoxDecoration(
                  color: cardBg,
                  borderRadius: const BorderRadius.vertical(bottom: Radius.circular(16)),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    _buildToolbarButton(
                      icon: Icons.folder_open_outlined,
                      label: '文件传输',
                      onTap: () => _onConnect(context, isFileTransfer: true),
                    ),
                    _buildDivider(isDark),
                    _buildToolbarButton(
                      icon: Icons.play_arrow_outlined,
                      label: '观看模式',
                      onTap: () => _onConnect(context),
                    ),
                    _buildDivider(isDark),
                    _buildToolbarButton(
                      icon: Icons.terminal_outlined,
                      label: '终端',
                      onTap: () => _onConnect(context, isTerminal: true),
                    ),
                    _buildDivider(isDark),
                    _buildToolbarButton(
                      icon: Icons.compare_arrows_outlined,
                      label: '端口映射',
                      onTap: () => _onConnect(context, isTcpTunneling: true),
                    ),
                    _buildDivider(isDark),
                    _buildToolbarButton(
                      icon: Icons.grid_view_outlined,
                      label: '更多功能',
                      onTap: () => _onConnect(context),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 28),

            // 快速启动区域
            const Text(
              '快速启动',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 12),

            // 虚线边框卡片与快捷指令
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 18),
              decoration: BoxDecoration(
                color: cardBg,
                borderRadius: BorderRadius.circular(14),
                border: Border.all(
                  color: borderColor,
                  width: 1.2,
                ),
              ),
              child: Wrap(
                spacing: 14,
                runSpacing: 12,
                children: [
                  _InteractiveQuickChip(
                    icon: Icons.lock_outline,
                    label: '一键锁屏',
                    color: const Color(0xFF6366F1),
                    onTap: () => _sendQuickAction(context, 'lock'),
                  ),
                  _InteractiveQuickChip(
                    icon: Icons.restart_alt,
                    label: '重启电脑',
                    color: const Color(0xFFEF4444),
                    onTap: () => _sendQuickAction(context, 'restart'),
                  ),
                  _InteractiveQuickChip(
                    icon: Icons.speed,
                    label: '任务管理器',
                    color: const Color(0xFF10B981),
                    onTap: () => _sendQuickAction(context, 'taskmgr'),
                  ),
                  _InteractiveQuickChip(
                    icon: Icons.keyboard_command_key,
                    label: 'Ctrl+Alt+Del',
                    color: const Color(0xFFF59E0B),
                    onTap: () => _sendQuickAction(context, 'cad'),
                  ),
                  // ➕ 添加卡片 (带交互微动效)
                  _InteractiveAddChip(
                    borderColor: borderColor,
                    isDark: isDark,
                    primaryColor: primaryColor,
                    onTap: () => showToast('已开启快速操作快捷方式'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildToolbarButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return _InteractiveToolbarButton(
      icon: icon,
      label: label,
      onTap: onTap,
    );
  }

  Widget _buildDivider(bool isDark) {
    return Container(
      width: 1,
      height: 18,
      color: isDark ? Colors.grey[800] : const Color(0xFFE2E8F0),
    );
  }
}

/// Win11 Bloom 风格花瓣曲线绘制器
class _Win11BloomPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..style = PaintingStyle.fill
      ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 30);

    // 绘制深蓝与亮青色光晕
    paint.color = const Color(0xFF38BDF8).withOpacity(0.35);
    canvas.drawCircle(Offset(size.width * 0.7, size.height * 0.4), size.height * 0.45, paint);

    paint.color = const Color(0xFF6366F1).withOpacity(0.35);
    canvas.drawCircle(Offset(size.width * 0.35, size.height * 0.6), size.height * 0.5, paint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

/// 具有悬浮深度、高亮边框和微动效的 16:9 桌面卡片
class _InteractivePreviewCard extends StatefulWidget {
  final Color cardBg;
  final Color borderColor;
  final bool isDark;
  final VoidCallback onTap;
  final Widget toolbar;

  const _InteractivePreviewCard({
    Key? key,
    required this.cardBg,
    required this.borderColor,
    required this.isDark,
    required this.onTap,
    required this.toolbar,
  }) : super(key: key);

  @override
  State<_InteractivePreviewCard> createState() => _InteractivePreviewCardState();
}

class _InteractivePreviewCardState extends State<_InteractivePreviewCard> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOutCubic,
        width: double.infinity,
        decoration: BoxDecoration(
          color: widget.cardBg,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(
            color: _isHovered
                ? const Color(0xFF1E6FFF).withOpacity(0.4)
                : widget.borderColor,
            width: _isHovered ? 1.2 : 1,
          ),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withOpacity(
                widget.isDark ? 0.35 : (_isHovered ? 0.12 : 0.05),
              ),
              blurRadius: _isHovered ? 24 : 16,
              offset: Offset(0, _isHovered ? 8 : 5),
            ),
          ],
        ),
        child: Column(
          children: [
            // 16:9 桌面卡片
            InkWell(
              onTap: widget.onTap,
              borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
              child: SizedBox(
                height: 240,
                width: double.infinity,
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    // 背景渐变
                    Positioned.fill(
                      child: ClipRRect(
                        borderRadius: const BorderRadius.vertical(top: Radius.circular(15)),
                        child: AnimatedContainer(
                          duration: const Duration(milliseconds: 300),
                          decoration: BoxDecoration(
                            gradient: LinearGradient(
                              colors: _isHovered
                                  ? [const Color(0xFF0F172A), const Color(0xFF1E40AF), const Color(0xFF3B82F6)]
                                  : [const Color(0xFF0F172A), const Color(0xFF1E3A8A), const Color(0xFF2563EB)],
                              begin: Alignment.topLeft,
                              end: Alignment.bottomRight,
                            ),
                          ),
                          child: CustomPaint(
                            painter: _Win11BloomPainter(),
                          ),
                        ),
                      ),
                    ),
                    // 居中远程桌面控制提示
                    AnimatedScale(
                      scale: _isHovered ? 1.05 : 1.0,
                      duration: const Duration(milliseconds: 180),
                      curve: Curves.easeOutCubic,
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12),
                        decoration: BoxDecoration(
                          color: Colors.black.withOpacity(_isHovered ? 0.55 : 0.4),
                          borderRadius: BorderRadius.circular(30),
                          border: Border.all(
                            color: Colors.white.withOpacity(_isHovered ? 0.6 : 0.3),
                            width: 1.2,
                          ),
                          boxShadow: [
                            BoxShadow(
                              color: const Color(0xFF1E6FFF).withOpacity(_isHovered ? 0.4 : 0.1),
                              blurRadius: _isHovered ? 16 : 8,
                            ),
                          ],
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(
                              Icons.play_circle_fill,
                              color: _isHovered ? const Color(0xFF60A5FA) : Colors.white,
                              size: 24,
                            ),
                            const SizedBox(width: 10),
                            const Text(
                              '点击直接进入远程桌面',
                              style: TextStyle(
                                color: Colors.white,
                                fontSize: 14,
                                fontWeight: FontWeight.w600,
                                letterSpacing: 0.5,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),

            // 快捷工具栏
            widget.toolbar,
          ],
        ),
      ),
    );
  }
}

/// 快捷工具栏交互按钮
class _InteractiveToolbarButton extends StatefulWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _InteractiveToolbarButton({
    Key? key,
    required this.icon,
    required this.label,
    required this.onTap,
  }) : super(key: key);

  @override
  State<_InteractiveToolbarButton> createState() => _InteractiveToolbarButtonState();
}

class _InteractiveToolbarButtonState extends State<_InteractiveToolbarButton> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    const activeColor = Color(0xFF1E6FFF);

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: InkWell(
        onTap: widget.onTap,
        borderRadius: BorderRadius.circular(8),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 160),
          curve: Curves.easeOutCubic,
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
          decoration: BoxDecoration(
            color: _isHovered ? activeColor.withOpacity(0.08) : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                widget.icon,
                size: 18,
                color: _isHovered ? activeColor : const Color(0xFF64748B),
              ),
              const SizedBox(width: 8),
              Text(
                widget.label,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: _isHovered ? activeColor : const Color(0xFF475569),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 快速启动芯片 (带有悬浮缩放与点击触感动效)
class _InteractiveQuickChip extends StatefulWidget {
  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  const _InteractiveQuickChip({
    Key? key,
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  }) : super(key: key);

  @override
  State<_InteractiveQuickChip> createState() => _InteractiveQuickChipState();
}

class _InteractiveQuickChipState extends State<_InteractiveQuickChip> {
  bool _isHovered = false;
  bool _isPressed = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() {
        _isHovered = false;
        _isPressed = false;
      }),
      child: GestureDetector(
        onTapDown: (_) => setState(() => _isPressed = true),
        onTapUp: (_) => setState(() => _isPressed = false),
        onTapCancel: () => setState(() => _isPressed = false),
        onTap: widget.onTap,
        child: AnimatedScale(
          scale: _isPressed ? 0.95 : (_isHovered ? 1.04 : 1.0),
          duration: const Duration(milliseconds: 120),
          curve: Curves.easeOutCubic,
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 180),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            decoration: BoxDecoration(
              color: widget.color.withOpacity(_isHovered ? 0.15 : 0.08),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: widget.color.withOpacity(_isHovered ? 0.45 : 0.2),
                width: 1,
              ),
              boxShadow: _isHovered
                  ? [
                      BoxShadow(
                        color: widget.color.withOpacity(0.18),
                        blurRadius: 8,
                        offset: const Offset(0, 2),
                      ),
                    ]
                  : null,
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(widget.icon, size: 16, color: widget.color),
                const SizedBox(width: 7),
                Text(
                  widget.label,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: widget.color,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 快速操作「添加」芯片
class _InteractiveAddChip extends StatefulWidget {
  final Color borderColor;
  final bool isDark;
  final Color primaryColor;
  final VoidCallback onTap;

  const _InteractiveAddChip({
    Key? key,
    required this.borderColor,
    required this.isDark,
    required this.primaryColor,
    required this.onTap,
  }) : super(key: key);

  @override
  State<_InteractiveAddChip> createState() => _InteractiveAddChipState();
}

class _InteractiveAddChipState extends State<_InteractiveAddChip> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: InkWell(
        onTap: widget.onTap,
        borderRadius: BorderRadius.circular(10),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          decoration: BoxDecoration(
            color: _isHovered
                ? widget.primaryColor.withOpacity(0.12)
                : (widget.isDark ? const Color(0xFF0F172A) : const Color(0xFFF1F5F9)),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(
              color: _isHovered ? widget.primaryColor.withOpacity(0.4) : widget.borderColor,
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.add, size: 18, color: widget.primaryColor),
              const SizedBox(width: 6),
              Text(
                '添加',
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: widget.primaryColor,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
