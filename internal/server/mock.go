package server

func mockTracks(seed string) []Track {
    stats := map[string]int{"collect_count":0,"comment_count":0,"share_count":0}
    return []Track{
        {ID:"mock_001", Title:"本地测试歌曲", Type:"audio", MediaKind:"audio", Artist:"Mouyin", Artists:[]string{"Mouyin"}, Album:seed, Duration:180000, Pic:"https://picsum.photos/seed/mock001/512/512", PicBG:"https://picsum.photos/seed/mock001/512/512", PlayURL:"", AudioURL:"", LyricsLRC:"[00:00.00]本地测试歌曲", LyricsType:"lrc", Source:"mock", Stats:stats, Tags:[]string{"mock"}, Description:"mock fallback"},
    }
}
