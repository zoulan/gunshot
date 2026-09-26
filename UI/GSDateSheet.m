#import "GSDateSheet.h"
#import "../Shared/GSLocalization.h"
@implementation GSDateSheet {
 UIDatePicker *_picker;
}
- (void)viewDidLoad{
 [super viewDidLoad];self.view.backgroundColor=UIColor.systemGroupedBackgroundColor;
 self.title=GSL(@"Choose a start date");
 self.navigationItem.leftBarButtonItem=[[UIBarButtonItem alloc]initWithTitle:GSL(@"Cancel") style:UIBarButtonItemStylePlain target:self action:@selector(close)];
 self.navigationItem.rightBarButtonItem=[[UIBarButtonItem alloc]initWithTitle:GSL(@"Continue") style:UIBarButtonItemStyleDone target:self action:@selector(confirm)];
 UIDatePicker *picker=[UIDatePicker new];
 picker.datePickerMode=UIDatePickerModeDate;
 picker.preferredDatePickerStyle=UIDatePickerStyleWheels;
 picker.maximumDate=[NSDate date];
 picker.date=[NSCalendar.currentCalendar startOfDayForDate:[NSDate date]];
 picker.translatesAutoresizingMaskIntoConstraints=NO;
 [self.view addSubview:picker];
 [NSLayoutConstraint activateConstraints:@[
  [picker.leadingAnchor constraintEqualToAnchor:self.view.safeAreaLayoutGuide.leadingAnchor],
  [picker.trailingAnchor constraintEqualToAnchor:self.view.safeAreaLayoutGuide.trailingAnchor],
  [picker.topAnchor constraintEqualToAnchor:self.view.safeAreaLayoutGuide.topAnchor],
  [picker.bottomAnchor constraintEqualToAnchor:self.view.safeAreaLayoutGuide.bottomAnchor]
 ]];
 _picker=picker;
}
- (void)close{[self dismissViewControllerAnimated:YES completion:nil];}
- (void)confirm{
 NSDate *date=_picker.date;
 void(^selection)(NSDate *)=self.selection;
 [self dismissViewControllerAnimated:YES completion:^{if(selection)selection(date);}];
}
@end
