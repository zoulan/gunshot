#import "GSBatchImport.h"
#import "GSExporter.h"
#import "GSNativeAccount.h"
#import "../Shared/IPCProtocol.h"
#import "../Shared/GSBackupPolicy.h"

@interface GSImportBatch : NSObject
@property(atomic,copy) NSString *stopReason;
@property(nonatomic,copy) NSString *account;
@property(nonatomic,copy) NSString *identity;
@end
@implementation GSImportBatch @end
static GSImportBatch *GSCurrentBatch;
static NSDictionary *GSLastBatch;
static dispatch_queue_t GSBatchQueue;

NSDictionary *GSBatchImportSnapshot(void){
 @synchronized(GSImportBatch.class){return GSLastBatch?:@{@"active":@NO};}
}
void GSStopBatchImport(BOOL backgroundExpired){
 @synchronized(GSImportBatch.class){GSCurrentBatch.stopReason=backgroundExpired?@"background_expired":@"cancelled";}
}
static void GSRecordBatch(NSDictionary *snapshot){@synchronized(GSImportBatch.class){GSLastBatch=[snapshot copy];}}
static NSString *GSCheckBatchAccount(GSImportBatch *batch){
 if(batch.stopReason)return batch.stopReason;
 if(batch.identity){
  __block BOOL valid=NO;
  dispatch_sync(dispatch_get_main_queue(),^{valid=GSNativeIdentityMatches(batch.identity);});
  if(!valid)return @"account_changed";
 }
 NSDictionary *accounts=GSRequest(@{@"op":@"accounts"},nil);
 if(!accounts)return @"service_unavailable";
 return [accounts[@"selected"]isEqual:batch.account]?nil:@"account_changed";
}
BOOL GSStartBatchImport(NSUInteger count,NSString *source,BOOL assets,GSBatchItemProvider provider,
 NSString *account,NSString *identity,GSBatchProgress progress,GSBatchProgress completion){
 NSCAssert(NSThread.isMainThread,@"Start import on main");
 if(!count||!provider||!account.length)return NO;
 GSImportBatch *batch=[GSImportBatch new];batch.account=[account copy];batch.identity=[identity copy];
 source=source&&[@[@"picker",@"album",@"share"]containsObject:source]?source:@"share";
 NSMutableDictionary *state=[@{@"active":@YES,@"source":source,@"total":@(count),@"processed":@0,@"queued":@0,@"failed":@0,@"skipped":@0,@"remaining":@(count),@"stage":@"starting"}mutableCopy];
 @synchronized(GSImportBatch.class){
  if(GSCurrentBatch)return NO;
  GSCurrentBatch=batch;GSLastBatch=[state copy];
  if(!GSBatchQueue)GSBatchQueue=dispatch_queue_create("dev.tqmane.gunshot.batch-import",DISPATCH_QUEUE_SERIAL);
 }
 dispatch_async(GSBatchQueue,^{@autoreleasepool{
  NSString *reason=GSCheckBatchAccount(batch);
  NSDictionary *options=reason?nil:GSRequest(@{@"op":@"options"},nil);
  if(!reason&&!options)reason=@"service_unavailable";
  NSString *quality=options[@"quality"]?:@"original";
  NSUInteger processed=0,queued=0,failed=0,skipped=0;
  NSMutableDictionary *failures=[NSMutableDictionary dictionary];
  NSTimeInterval lastUpdate=0;
  for(NSUInteger index=0;index<count&&!reason;index++){@autoreleasepool{
   reason=GSCheckBatchAccount(batch);if(reason)break;
   state[@"stage"]=@"exporting";GSRecordBatch(state);
   id item=provider(index);NSError *error=nil;
   if(!item){processed++;failed++;failures[@"inaccessible"]=@([failures[@"inaccessible"]unsignedIntegerValue]+1);}
   else {
    NSURL *directory=[NSURL fileURLWithPath:[NSTemporaryDirectory()stringByAppendingPathComponent:NSUUID.UUID.UUIDString] isDirectory:YES];
    BOOL created=[NSFileManager.defaultManager createDirectoryAtURL:directory withIntermediateDirectories:YES attributes:@{NSFilePosixPermissions:@0700} error:&error];
    NSArray *files=nil;NSDate *date=nil;BOOL scoped=NO;
    if(!created)reason=@"local_storage";
    else if(assets){PHAsset *asset=item;date=asset.creationDate;
     NSDate *since=GSBackupSinceDate();
     // Skip before exporting: old albums would otherwise pay full export cost.
     if(since&&date&&[date compare:since]==NSOrderedAscending)files=@[];
     else files=GSExportAsset(asset,directory,&error);}
    else {NSURL *url=item;scoped=[url startAccessingSecurityScopedResource];NSDictionary *attr=[NSFileManager.defaultManager attributesOfItemAtPath:url.path error:&error];if(attr){files=@[url];date=attr[NSFileModificationDate];
     NSDate *since=GSBackupSinceDate();
     if(since&&date&&[date compare:since]==NSOrderedAscending)files=@[];}}
    if(!reason)reason=GSCheckBatchAccount(batch); // Cloud export may outlive sign-in or cancellation.
    if(!reason&&!files){
     if([error.domain isEqual:NSCocoaErrorDomain]&&error.code==NSFileWriteOutOfSpaceError)reason=@"local_storage";
     else {processed++;failed++;failures[@"export_failed"]=@([failures[@"export_failed"]unsignedIntegerValue]+1);}
    }
    if(!reason&&files){
     state[@"stage"]=@"queueing";GSRecordBatch(state);
     NSString *job=GSImportFiles(files,batch.account,quality,date,&error);
     if(job){processed++;queued++;}else reason=@"queue_rejected";
    }else if(files)skipped++; // Empty sentinel array: item predates the start date.
    if(scoped)[item stopAccessingSecurityScopedResource];
    [NSFileManager.defaultManager removeItemAtURL:directory error:nil];
   }
   state[@"processed"]=@(processed);state[@"queued"]=@(queued);state[@"failed"]=@(failed);state[@"skipped"]=@(skipped);state[@"remaining"]=@(count-processed);state[@"failureCodes"]=[failures copy];GSRecordBatch(state);
   NSTimeInterval now=NSProcessInfo.processInfo.systemUptime;
   if(progress&&now-lastUpdate>=0.25){lastUpdate=now;NSDictionary *snapshot=[state copy];dispatch_async(dispatch_get_main_queue(),^{progress(snapshot);});}
  }}
  state[@"active"]=@NO;state[@"stage"]=reason?@"stopped":@"finished";
  if(reason)state[@"stopReason"]=reason;
  NSDictionary *final=[state copy];
  // Release the reservation before notifying UI, allowing an explicit retry.
  @synchronized(GSImportBatch.class){GSLastBatch=final;GSCurrentBatch=nil;}
  if(completion)dispatch_async(dispatch_get_main_queue(),^{completion(final);});
 }});
 return YES;
}
GSBatchItemProvider GSPhotoIdentifierProvider(NSArray *identifiers){
 NSArray *selection=[identifiers copy];
 __block NSUInteger pageStart=NSNotFound;
 __block NSDictionary *page;
 return ^id(NSUInteger index){
  NSCAssert(!NSThread.isMainThread,@"Resolve selected photos off main");
  if(index>=selection.count)return nil;
  NSUInteger start=index/64*64;
  if(start!=pageStart){
   NSMutableArray *ids=[NSMutableArray array];
   for(id value in [selection subarrayWithRange:NSMakeRange(start,MIN((NSUInteger)64,selection.count-start))])
    if([value isKindOfClass:NSString.class]&&[value length])[ids addObject:value];
   NSMutableDictionary *resolved=[NSMutableDictionary dictionary];
   PHFetchResult *found=[PHAsset fetchAssetsWithLocalIdentifiers:ids options:nil];
   [found enumerateObjectsUsingBlock:^(PHAsset *asset,NSUInteger i,BOOL *stop){resolved[asset.localIdentifier]=asset;}];
   page=[resolved copy];pageStart=start;
  }
  id key=selection[index];return [key isKindOfClass:NSString.class]?page[key]:nil;
 };
}
