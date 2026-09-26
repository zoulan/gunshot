#pragma once
#import <Foundation/Foundation.h>

// Backup start date policy: when set, only photos and videos taken on or
// after this date are queued. Older items are skipped at import time.
static inline NSDate *GSBackupSinceDate(void) {
	return [NSUserDefaults.standardUserDefaults objectForKey:@"dev.tqmane.gunshot.backup-since"];
}
static inline void GSSetBackupSinceDate(NSDate *date) {
	if (date) [NSUserDefaults.standardUserDefaults setObject:date forKey:@"dev.tqmane.gunshot.backup-since"];
	else [NSUserDefaults.standardUserDefaults removeObjectForKey:@"dev.tqmane.gunshot.backup-since"];
}
