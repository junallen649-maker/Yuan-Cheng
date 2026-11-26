import 'package:flutter_hbb/models/peer_tab_utils.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const recentIndex = 0;
  const favIndex = 1;
  const lanIndex = 2;
  const abIndex = 3;
  const groupIndex = 4;

  test('switches from recent, fav, or lan to group on desktop login', () {
    for (final tab in [recentIndex, favIndex, lanIndex]) {
      expect(
        resolveDesktopLoginPeerTabIndex(
          currentTab: tab,
          abTabIndex: abIndex,
          groupTabIndex: groupIndex,
          isGroupVisibleEnabled: true,
          isTabIndexFixed: false,
          isDesktopPlatform: true,
        ),
        groupIndex,
      );
    }
  });

  test('keeps current tab when already on ab or group', () {
    for (final tab in [abIndex, groupIndex]) {
      expect(
        resolveDesktopLoginPeerTabIndex(
          currentTab: tab,
          abTabIndex: abIndex,
          groupTabIndex: groupIndex,
          isGroupVisibleEnabled: true,
          isTabIndexFixed: false,
          isDesktopPlatform: true,
        ),
        isNull,
      );
    }
  });

  test('does not switch when group tab is hidden/disabled, fixed, or non-desktop', () {
    expect(
      resolveDesktopLoginPeerTabIndex(
        currentTab: recentIndex,
        abTabIndex: abIndex,
        groupTabIndex: groupIndex,
        isGroupVisibleEnabled: false,
        isTabIndexFixed: false,
        isDesktopPlatform: true,
      ),
      isNull,
    );
    expect(
      resolveDesktopLoginPeerTabIndex(
        currentTab: recentIndex,
        abTabIndex: abIndex,
        groupTabIndex: groupIndex,
        isGroupVisibleEnabled: true,
        isTabIndexFixed: true,
        isDesktopPlatform: true,
      ),
      isNull,
    );
    expect(
      resolveDesktopLoginPeerTabIndex(
        currentTab: recentIndex,
        abTabIndex: abIndex,
        groupTabIndex: groupIndex,
        isGroupVisibleEnabled: true,
        isTabIndexFixed: false,
        isDesktopPlatform: false,
      ),
      isNull,
    );
  });
}
