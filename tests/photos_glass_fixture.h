// Included by the existing settings UIKit smoke. The production feature finds
// these classes by their Google Photos runtime names, so keep the fake surface
// limited to the audited contracts used by GSPhotosGlass.m.
#import "../UI/GSPhotosGlass.h"
#import <UIKit/UIKit.h>
#import <objc/runtime.h>
#include <string.h>

@interface PHSSegmentedControl : UIControl {
 NSInteger _selectedSegmentIndex;
}
@property(nonatomic) NSInteger selectedSegmentIndex;
@property(nonatomic,strong) UIView *selection;
- (NSInteger)numberOfSegments;
@end
@implementation PHSSegmentedControl
- (instancetype)initWithFrame:(CGRect)frame{
 if((self=[super initWithFrame:frame])){
  self.backgroundColor=UIColor.secondarySystemBackgroundColor;self.opaque=YES;
  self.selection=[[UIView alloc]initWithFrame:CGRectMake(4,4,80,40)];self.selection.backgroundColor=UIColor.tertiarySystemFillColor;[self addSubview:self.selection];
  NSArray *titles=@[@"Photos",@"Collections",@"Create"];
  for(NSInteger i=0;i<3;i++){
   UILabel *label=[[UILabel alloc]initWithFrame:CGRectMake(8+i*84,4,76,40)];label.text=titles[i];label.textAlignment=NSTextAlignmentCenter;[self addSubview:label];
  }
 }
 return self;
}
- (NSInteger)numberOfSegments{return 3;}
- (NSInteger)selectedSegmentIndex{return _selectedSegmentIndex;}
- (void)setSelectedSegmentIndex:(NSInteger)value{
 if(_selectedSegmentIndex==value)return;_selectedSegmentIndex=value;[self sendActionsForControlEvents:UIControlEventValueChanged];
}
- (void)layoutSubviews{
 [super layoutSubviews];self.selection.frame=CGRectMake(4,4,MIN(80,MAX(0,self.bounds.size.width-8)),MAX(0,self.bounds.size.height-8));
}
@end

@interface M3CButton : UIButton
@end
@implementation M3CButton
- (void)layoutSubviews{[super layoutSubviews];}
@end

@interface PHSTabBarController : UIViewController
@property(nonatomic,strong) UIStackView *floatingBottomTabBar;
@property(nonatomic,strong) PHSSegmentedControl *floatingSegmentedControl;
@property(nonatomic,strong) M3CButton *floatingSearchButton;
@property(nonatomic,strong) UITapGestureRecognizer *fixtureGesture;
@property(nonatomic) NSUInteger taps;
@end
@implementation PHSTabBarController
- (void)tap:(id)sender{self.taps++;}
- (void)gestureTap:(UITapGestureRecognizer *)gesture{self.taps++;}
- (void)viewDidLoad{
 [super viewDidLoad];self.view.backgroundColor=UIColor.systemBackgroundColor;
 // Deliberately make Google's source segmented control thinner than Search. The
 // native overlay must ignore this compact source geometry and use the system's
 // full-screen UITabBarController layout instead.
 self.floatingSegmentedControl=[[PHSSegmentedControl alloc]initWithFrame:CGRectMake(0,0,260,44)];
 self.floatingSearchButton=[[M3CButton alloc]initWithFrame:CGRectMake(0,0,56,56)];
 self.floatingSearchButton.accessibilityLabel=@"Search";[self.floatingSearchButton setImage:[UIImage systemImageNamed:@"magnifyingglass"] forState:UIControlStateNormal];
 [self.floatingSearchButton addTarget:self action:@selector(tap:) forControlEvents:UIControlEventTouchUpInside];
 [self.floatingSegmentedControl addTarget:self action:@selector(tap:) forControlEvents:UIControlEventValueChanged];
 self.fixtureGesture=[[UITapGestureRecognizer alloc]initWithTarget:self action:@selector(gestureTap:)];[self.floatingSearchButton addGestureRecognizer:self.fixtureGesture];
 self.floatingBottomTabBar=[[UIStackView alloc]initWithArrangedSubviews:@[self.floatingSegmentedControl,self.floatingSearchButton]];
 self.floatingBottomTabBar.axis=UILayoutConstraintAxisHorizontal;self.floatingBottomTabBar.spacing=12;[self.view addSubview:self.floatingBottomTabBar];
}
- (void)viewDidLayoutSubviews{
 [super viewDidLayoutSubviews];CGFloat width=MIN(MAX(220,self.view.bounds.size.width-32),360);
 self.floatingBottomTabBar.frame=CGRectMake(16,MAX(0,self.view.bounds.size.height-76),width,60);
 self.floatingSegmentedControl.frame=CGRectMake(0,8,MAX(120,width-72),44);self.floatingSearchButton.frame=CGRectMake(MAX(0,width-60),2,56,56);
 [self.floatingSegmentedControl layoutIfNeeded];[self.floatingSearchButton layoutIfNeeded];
}
@end

static id (*GSOriginalBundleInfo)(id,SEL,id);
static id GSGlassFixturePhotosVersion;
static id GSGlassBundleInfo(id bundle,SEL selector,id key){
 if(bundle==NSBundle.mainBundle){
  if([key isEqual:@"CFBundleExecutable"])return @"GooglePhotos";
  if([key isEqual:@"CFBundleShortVersionString"])return GSGlassFixturePhotosVersion;
 }
 return GSOriginalBundleInfo(bundle,selector,key);
}
static BOOL GSFixtureABI(Class cls,NSString *name,const char *abi){
 Method method=class_getInstanceMethod(cls,NSSelectorFromString(name));return method&&!strcmp(method_getTypeEncoding(method),abi);
}
static BOOL GSFixturePhotosGlassContracts(void){
 for(NSArray *entry in @[
  @[@"PHSTabBarController",@"viewDidLayoutSubviews",@"v16@0:8"],
  @[@"PHSTabBarController",@"floatingBottomTabBar",@"@16@0:8"],
  @[@"PHSTabBarController",@"floatingSegmentedControl",@"@16@0:8"],
  @[@"PHSTabBarController",@"floatingSearchButton",@"@16@0:8"],
  @[@"PHSSegmentedControl",@"layoutSubviews",@"v16@0:8"],
  @[@"PHSSegmentedControl",@"numberOfSegments",@"q16@0:8"],
  @[@"PHSSegmentedControl",@"selectedSegmentIndex",@"q16@0:8"],
  @[@"PHSSegmentedControl",@"setSelectedSegmentIndex:",@"v24@0:8q16"],
  @[@"M3CButton",@"layoutSubviews",@"v16@0:8"]])
  if(!GSFixtureABI(NSClassFromString(entry[0]),entry[1],[entry[2]UTF8String]))return NO;
 return YES;
}

static void GSFixtureAttach(PHSTabBarController *controller,UIWindow *window){
 UIViewController *root=window.rootViewController;[controller loadViewIfNeeded];controller.view.frame=window.bounds;
 [root addChildViewController:controller];[root.view addSubview:controller.view];[controller didMoveToParentViewController:root];[controller.view setNeedsLayout];[controller.view layoutIfNeeded];
}
static void GSFixtureDetach(PHSTabBarController *controller){
 [controller willMoveToParentViewController:nil];[controller.view removeFromSuperview];[controller removeFromParentViewController];
}
static UIWindow *GSFixtureGlassOverlayWindow(UIWindow *host){
 if(!host.windowScene)return nil;
 for(UIWindow *candidate in host.windowScene.windows){
  if(candidate==host||candidate.hidden||!candidate.rootViewController)continue;
  if([NSStringFromClass(candidate.class)isEqualToString:@"GSPhotosGlassOverlayWindow"]&&[candidate.rootViewController isKindOfClass:UITabBarController.class])return candidate;
 }
 return nil;
}

#define GS_GLASS_CHECK(value) do{if(!(value)){NSLog(@"FAIL bottom glass: %s",#value);return NO;}}while(0)
static BOOL GSCheckPhotosGlass(GSPanel *panel,UIWindow *window){
 BOOL modern=NO;if(@available(iOS 26.0,*))modern=YES;
 Method info=class_getInstanceMethod(NSBundle.class,@selector(objectForInfoDictionaryKey:));GSOriginalBundleInfo=(void *)method_setImplementation(info,(IMP)GSGlassBundleInfo);
 @try{
  GS_GLASS_CHECK(GSFixturePhotosGlassContracts());
  for(id version in @[@"7.20.2",@"7.91.9",@"7.9.20",@"unknown",@"",@"7.92.0-beta",@"7.92.0.1",@42]){GSGlassFixturePhotosVersion=version;GS_GLASS_CHECK(!GSPhotosGlassAvailable());}
  for(NSString *version in @[@"7.92",@"7.92.0",@"7.100.0",@"8.0.0"]){GSGlassFixturePhotosVersion=version;GS_GLASS_CHECK(GSPhotosGlassAvailable()==modern);}
  GSGlassFixturePhotosVersion=@"7.92.0";[NSUserDefaults.standardUserDefaults setBool:NO forKey:@"GSPhotosBottomBarLiquidGlass"];

  UITableViewCell *cell=[panel tableView:panel.tableView cellForRowAtIndexPath:[NSIndexPath indexPathForRow:2 inSection:6]];UISwitch *toggle=(UISwitch *)cell.accessoryView;
  GS_GLASS_CHECK([cell.textLabel.text isEqual:@"Google Photos · Liquid Glass"]&&[toggle isKindOfClass:UISwitch.class]&&!toggle.on&&toggle.enabled==modern);
  if(!modern){GS_GLASS_CHECK(!GSPhotosGlassEnabled());return YES;}

  // The settings sheet stays presented over the fixture's main window for the whole
  // test, and the overlay intentionally hides while anything is presented over its
  // host window. Run the overlay checks in a dedicated bare window so they model the
  // real Photos main window, which has no modal while the bottom bar is visible.
  UIWindow *host=[[UIWindow alloc]initWithWindowScene:window.windowScene];
  host.frame=window.windowScene.coordinateSpace.bounds;
  host.rootViewController=[UIViewController new];host.hidden=NO;

  PHSTabBarController *controller=[PHSTabBarController new];GSFixtureAttach(controller,host);
  PHSSegmentedControl *segments=controller.floatingSegmentedControl;M3CButton *search=controller.floatingSearchButton;UIStackView *bar=controller.floatingBottomTabBar;
  UIView *selection=segments.selection;UIImage *glyph=[search imageForState:UIControlStateNormal];UITapGestureRecognizer *gesture=controller.fixtureGesture;
  NSUInteger hostChildren=controller.childViewControllers.count;
  GS_GLASS_CHECK(!GSPhotosGlassEnabled()&&!GSFixtureGlassOverlayWindow(host)&&segments.alpha==1&&search.alpha==1);
  GS_GLASS_CHECK([search.allTargets containsObject:controller]&&[segments.allTargets containsObject:controller]&&[search.gestureRecognizers containsObject:gesture]);

  toggle.on=YES;[toggle sendActionsForControlEvents:UIControlEventValueChanged];GS_GLASS_CHECK(GSPhotosGlassEnabled());GSInstallPhotosGlass();GSInstallPhotosGlass();[controller viewDidLayoutSubviews];
  NSDictionary *snapshot=GSPhotosGlassSnapshot();GS_GLASS_CHECK([snapshot[@"available"]boolValue]&&[snapshot[@"hooksInstalled"]boolValue]&&[snapshot[@"attachedBars"]unsignedIntegerValue]>=1);
  GS_GLASS_CHECK([snapshot[@"designCompatibilityOverride"]boolValue]&&![snapshot[@"restartRequired"]boolValue]&&[snapshot[@"activeThisLaunch"]boolValue]);

  UIWindow *overlay=GSFixtureGlassOverlayWindow(host);UITabBarController *nativeTabs=(UITabBarController *)overlay.rootViewController;
  GS_GLASS_CHECK(overlay&&nativeTabs&&nativeTabs.parentViewController==nil&&controller.childViewControllers.count==hostChildren);
  GS_GLASS_CHECK(nativeTabs.delegate&&nativeTabs.mode==UITabBarControllerModeTabBar&&nativeTabs.tabs.count==4&&nativeTabs.tabBar.window==overlay&&!nativeTabs.view.hidden);
  GS_GLASS_CHECK([nativeTabs.tabs[0].title isEqual:@"Photos"]&&[nativeTabs.tabs[1].title isEqual:@"Collections"]&&[nativeTabs.tabs[2].title isEqual:@"Create"]);
  GS_GLASS_CHECK([nativeTabs.tabs[3] isKindOfClass:NSClassFromString(@"UISearchTab")]);
  GS_GLASS_CHECK(CGRectEqualToRect(overlay.frame,host.windowScene.coordinateSpace.bounds)&&CGRectGetHeight(nativeTabs.view.bounds)>CGRectGetHeight(bar.bounds)*3.0);
  [nativeTabs.view setNeedsLayout];[nativeTabs.view layoutIfNeeded];[nativeTabs.tabBar layoutIfNeeded];
  CGRect nativeHit=[nativeTabs.tabBar convertRect:nativeTabs.tabBar.bounds toView:overlay];
  GS_GLASS_CHECK(CGRectGetHeight(nativeHit)>0&&[overlay pointInside:CGPointMake(CGRectGetMidX(nativeHit),CGRectGetMidY(nativeHit)) withEvent:nil]);
  GS_GLASS_CHECK(![overlay pointInside:CGPointMake(4,4) withEvent:nil]);
  GS_GLASS_CHECK(segments.superview==bar&&search.superview==bar&&segments.alpha==0&&search.alpha==0&&!segments.userInteractionEnabled&&!search.userInteractionEnabled);
  GS_GLASS_CHECK(segments.accessibilityElementsHidden&&search.accessibilityElementsHidden&&segments.selection==selection&&[search imageForState:UIControlStateNormal]==glyph);
  GS_GLASS_CHECK([search.allTargets containsObject:controller]&&[segments.allTargets containsObject:controller]&&[search.gestureRecognizers containsObject:gesture]);

  id<UITabBarControllerDelegate> delegate=nativeTabs.delegate;NSUInteger taps=controller.taps;UITab *createTab=nativeTabs.tabs[2];
  GS_GLASS_CHECK([delegate tabBarController:nativeTabs shouldSelectTab:createTab]);nativeTabs.selectedTab=createTab;
  GS_GLASS_CHECK(controller.taps==taps+1&&segments.selectedSegmentIndex==2&&nativeTabs.selectedTab==createTab);
  taps=controller.taps;GS_GLASS_CHECK([delegate tabBarController:nativeTabs shouldSelectTab:createTab]&&controller.taps==taps);

  taps=controller.taps;UITab *searchTab=nativeTabs.tabs[3];GS_GLASS_CHECK(![delegate tabBarController:nativeTabs shouldSelectTab:searchTab]&&controller.taps==taps+1);
  [controller viewDidLayoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&!nativeTabs.view.hidden&&nativeTabs.selectedTab==createTab);

  segments.selectedSegmentIndex=1;GS_GLASS_CHECK(nativeTabs.selectedTab==nativeTabs.tabs[1]&&segments.selection==selection);
  bar.alpha=0;[controller viewDidLayoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&nativeTabs.view.hidden);
  bar.alpha=1;[controller viewDidLayoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&!nativeTabs.view.hidden);

  UIView *occluder=[[UIView alloc]initWithFrame:CGRectMake(0,MAX(0,host.bounds.size.height-140),host.bounds.size.width,140)];
  occluder.backgroundColor=UIColor.clearColor;occluder.userInteractionEnabled=YES;[host addSubview:occluder];
  [controller viewDidLayoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&nativeTabs.view.hidden&&[GSPhotosGlassSnapshot()[@"visibleOverlays"]unsignedIntegerValue]==0);
  [occluder removeFromSuperview];[controller viewDidLayoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&!nativeTabs.view.hidden&&[GSPhotosGlassSnapshot()[@"visibleOverlays"]unsignedIntegerValue]>=1);

  CGPoint barCenter=bar.center;bar.center=CGPointMake(barCenter.x,barCenter.y+160);[segments layoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&nativeTabs.view.hidden);
  bar.center=barCenter;[segments layoutSubviews];GS_GLASS_CHECK(!overlay.hidden&&!nativeTabs.view.hidden);

  GSSetPhotosGlass(NO);GSSetPhotosGlass(NO);
  GS_GLASS_CHECK(!GSPhotosGlassEnabled()&&overlay.hidden&&overlay.rootViewController==nil);
  GS_GLASS_CHECK(segments.alpha==1&&segments.userInteractionEnabled&&!segments.accessibilityElementsHidden&&search.alpha==1&&search.userInteractionEnabled&&!search.accessibilityElementsHidden);
  GS_GLASS_CHECK(segments.selection==selection&&[search imageForState:UIControlStateNormal]==glyph&&[search.gestureRecognizers containsObject:gesture]);
  snapshot=GSPhotosGlassSnapshot();GS_GLASS_CHECK(![snapshot[@"enabled"]boolValue]&&[snapshot[@"attachedBars"]unsignedIntegerValue]==0);

  PHSTabBarController *bad=[PHSTabBarController new];[bad loadViewIfNeeded];[bad.floatingBottomTabBar removeArrangedSubview:bad.floatingSearchButton];[bad.floatingSearchButton removeFromSuperview];[bad.view addSubview:bad.floatingSearchButton];
  GSSetPhotosGlass(YES);[bad viewDidLayoutSubviews];GS_GLASS_CHECK([GSPhotosGlassSnapshot()[@"lastSkipReason"]isEqual:@"floating_bottom_bar_not_found"]);GSSetPhotosGlass(NO);

  GSFixtureDetach(controller);host.hidden=YES;host.rootViewController=nil;
  NSLog(@"PASS independent full-screen UITabBarController overlay with UITab + pinned UISearchTab, no Google child-controller insertion, source routing, passthrough hit-testing, occlusion/offscreen suppression, visibility mirroring and restoration");
 } @finally {
  method_setImplementation(info,(IMP)GSOriginalBundleInfo);GSGlassFixturePhotosVersion=nil;
 }
 GS_GLASS_CHECK(!GSPhotosGlassAvailable());return YES;
}
#undef GS_GLASS_CHECK