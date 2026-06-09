export namespace main {
	
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    downloadUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.downloadUrl = source["downloadUrl"];
	    }
	}

}

export namespace models {
	
	export class Achievement {
	    id: string;
	    name: string;
	    description: string;
	    iconUrl: string;
	    iconGrayUrl: string;
	    isAchieved: boolean;
	    unlockTime: number;
	    isHidden: boolean;
	    permission: number;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new Achievement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.iconUrl = source["iconUrl"];
	        this.iconGrayUrl = source["iconGrayUrl"];
	        this.isAchieved = source["isAchieved"];
	        this.unlockTime = source["unlockTime"];
	        this.isHidden = source["isHidden"];
	        this.permission = source["permission"];
	        this.percent = source["percent"];
	    }
	}
	export class GameInfo {
	    appId: number;
	    name: string;
	    logoUrl: string;
	    installed: boolean;
	    lastPlayed: number;
	    isSoftware: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.name = source["name"];
	        this.logoUrl = source["logoUrl"];
	        this.installed = source["installed"];
	        this.lastPlayed = source["lastPlayed"];
	        this.isSoftware = source["isSoftware"];
	    }
	}

}

export namespace services {
	
	export class HLTBTimes {
	    main: number;
	    mainExtra: number;
	    completionist: number;
	
	    static createFrom(source: any = {}) {
	        return new HLTBTimes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.main = source["main"];
	        this.mainExtra = source["mainExtra"];
	        this.completionist = source["completionist"];
	    }
	}

}

export namespace settings {
	
	export class Settings {
	    viewMode: string;
	    showLabels: boolean;
	    sortBy: string;
	    sortOrder: string;
	    installedOpen: boolean;
	    otherOpen: boolean;
	    autoStore: boolean;
	    allowLock: boolean;
	    showUnlockDates: boolean;
	    achievementSort: string;
	    achievementSortDir: string;
	    showSoftware: boolean;
	    showCardButtons: boolean;
	    protectLastPlayed: boolean;
	    cardMinWidth: number;
	    windowWidth: number;
	    windowHeight: number;
	    lastScanTime?: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.viewMode = source["viewMode"];
	        this.showLabels = source["showLabels"];
	        this.sortBy = source["sortBy"];
	        this.sortOrder = source["sortOrder"];
	        this.installedOpen = source["installedOpen"];
	        this.otherOpen = source["otherOpen"];
	        this.autoStore = source["autoStore"];
	        this.allowLock = source["allowLock"];
	        this.showUnlockDates = source["showUnlockDates"];
	        this.achievementSort = source["achievementSort"];
	        this.achievementSortDir = source["achievementSortDir"];
	        this.showSoftware = source["showSoftware"];
	        this.showCardButtons = source["showCardButtons"];
	        this.protectLastPlayed = source["protectLastPlayed"];
	        this.cardMinWidth = source["cardMinWidth"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.lastScanTime = source["lastScanTime"];
	    }
	}

}

