import {defineStore} from 'pinia'
import {ref} from "vue"
import type {appType} from "@/types/app"
import appApi from "@/api/app"

export const useIndexStore = defineStore("index-store", () => {
    const appInfo = ref<appType.App>({
        AppName: "",
        Version: "",
        Description: "",
        Copyright: "",
        Platform: "",
    })

    const globalConfig = ref<appType.Config>({
        Theme: "lightTheme",
        Locale: "zh",
        Host: "0.0.0.0",
        Port: "8899",
        Quality: 0,
        SaveDirectory: "",
        UpstreamProxy: "",
        FilenameLen: 0,
        FilenameTime: false,
        OpenProxy: false,
        DownloadProxy: false,
        AutoProxy: false,
        WxAction: false,
        TaskNumber: 8,
        UserAgent: "",
        UseHeaders: "",
        MimeMap: {}
    })

    const tableHeight = ref(800)

    const isProxy = ref(false)

    const init = async () => {
        await appApi.appInfo().then((res) => {
            appInfo.value = Object.assign({}, appInfo.value, res.data)
            isProxy.value = res.data.IsProxy
        })
        await appApi.getConfig().then((res) => {
            globalConfig.value = Object.assign({}, globalConfig.value, res.data)
        })
        setTimeout(() => {
            appApi.isProxy().then((res: appType.Res) => {
                isProxy.value = res.data.value
            })
        }, 150)
        window.addEventListener("resize", handleResize);
        handleResize()
    }

    const setConfig = (formValue: Object) => {
        globalConfig.value = Object.assign({}, globalConfig.value, formValue)
        appApi.setConfig(globalConfig.value)
    }

    const handleResize = () => {
        tableHeight.value = document.getElementById("content")?.offsetHeight ?? 895
    }

    const updateProxyStatus = (res: any) => {
        isProxy.value = res.value
    }

    const openProxy = async () => {
        return appApi.openSystemProxy().then(handleProxy)
    }

    const unsetProxy = async () => {
        return appApi.unsetSystemProxy().then(handleProxy)
    }

    const handleProxy = (res: appType.Res) => {
        isProxy.value = res.data.value
        if (res.code === 0) {
            window?.$message?.error(res.message)
        }
        return res
    }

    return {appInfo, globalConfig, tableHeight, isProxy, init, setConfig, openProxy, unsetProxy, updateProxyStatus}
})