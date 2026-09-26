#import <UIKit/UIKit.h>
typedef void (^GSDateSelection)(NSDate *date);
@interface GSDateSheet : UIViewController
@property(nonatomic,copy) GSDateSelection selection;
@end
