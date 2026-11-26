int? resolveDesktopLoginPeerTabIndex({
  required int currentTab,
  required int abTabIndex,
  required int groupTabIndex,
  required bool isGroupVisibleEnabled,
  required bool isTabIndexFixed,
  required bool isDesktopPlatform,
}) {
  if (!isDesktopPlatform || isTabIndexFixed || !isGroupVisibleEnabled) {
    return null;
  }
  if (currentTab == abTabIndex || currentTab == groupTabIndex) {
    return null;
  }
  return groupTabIndex;
}
