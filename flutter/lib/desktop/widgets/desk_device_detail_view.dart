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

    return SingleChildScrollView(
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

          // 主控预览大卡片 (16:9 比例 Win11 几何壁纸质感)
          Container(
            width: double.infinity,
            decoration: BoxDecoration(
              color: cardBg,
              borderRadius: BorderRadius.circular(16),
              border: Border.all(color: borderColor, width: 1),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withOpacity(isDark ? 0.25 : 0.05),
                  blurRadius: 16,
                  offset: const Offset(0, 6),
                ),
              ],
            ),
            child: Column(
              children: [
                // 16:9 桌面卡片
                InkWell(
                  onTap: () => _onConnect(context),
                  borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
                  child: Container(
                    height: 240,
                    width: double.infinity,
                    decoration: BoxDecoration(
                      borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
                      gradient: const LinearGradient(
                        colors: [Color(0xFF0F172A), Color(0xFF1E3A8A), Color(0xFF2563EB)],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                    ),
                    child: Stack(
                      alignment: Alignment.center,
                      children: [
                        // Win11 蓝浪潮几何视觉背景
                        Positioned.fill(
                          child: CustomPaint(
                            painter: _Win11BloomPainter(),
                          ),
                        ),
                        // 居中远程桌面控制提示
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 10),
                          decoration: BoxDecoration(
                            color: Colors.black.withOpacity(0.4),
                            borderRadius: BorderRadius.circular(30),
                            border: Border.all(color: Colors.white.withOpacity(0.3)),
                          ),
                          child: const Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(Icons.play_circle_fill, color: Colors.white, size: 24),
                              SizedBox(width: 8),
                              Text(
                                '点击直接进入远程桌面',
                                style: TextStyle(
                                  color: Colors.white,
                                  fontSize: 14,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),

                // 快捷工具栏 (文件传输 | 观看模式 | 终端 | 端口映射 | 更多)
                Container(
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
              ],
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
                _buildQuickActionChip(
                  icon: Icons.lock_outline,
                  label: '一键锁屏',
                  color: const Color(0xFF6366F1),
                  onTap: () => _sendQuickAction(context, 'lock'),
                ),
                _buildQuickActionChip(
                  icon: Icons.restart_alt,
                  label: '重启电脑',
                  color: const Color(0xFFEF4444),
                  onTap: () => _sendQuickAction(context, 'restart'),
                ),
                _buildQuickActionChip(
                  icon: Icons.speed,
                  label: '任务管理器',
                  color: const Color(0xFF10B981),
                  onTap: () => _sendQuickAction(context, 'taskmgr'),
                ),
                _buildQuickActionChip(
                  icon: Icons.keyboard_command_key,
                  label: 'Ctrl+Alt+Del',
                  color: const Color(0xFFF59E0B),
                  onTap: () => _sendQuickAction(context, 'cad'),
                ),
                // ➕ 添加卡片
                InkWell(
                  onTap: () => showToast('已开启快速操作快捷方式'),
                  borderRadius: BorderRadius.circular(10),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                    decoration: BoxDecoration(
                      color: isDark ? const Color(0xFF0F172A) : const Color(0xFFF1F5F9),
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(color: borderColor),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.add, size: 18, color: primaryColor),
                        const SizedBox(width: 6),
                        Text(
                          '添加',
                          style: TextStyle(
                            fontSize: 13,
                            fontWeight: FontWeight.w600,
                            color: primaryColor,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildToolbarButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 18, color: const Color(0xFF475569)),
            const SizedBox(width: 8),
            Text(
              label,
              style: const TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: Color(0xFF334155),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDivider(bool isDark) {
    return Container(
      width: 1,
      height: 18,
      color: isDark ? Colors.grey[800] : const Color(0xFFE2E8F0),
    );
  }

  Widget _buildQuickActionChip({
    required IconData icon,
    required String label,
    required Color color,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: color.withOpacity(0.08),
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: color.withOpacity(0.2)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 16, color: color),
            const SizedBox(width: 6),
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: color,
              ),
            ),
          ],
        ),
      ),
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
