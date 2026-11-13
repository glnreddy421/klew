import { Composition, registerRoot } from 'remotion'
import { KlewResourcesDemo, totalFrames } from './Video.jsx'
import { KlewInvestigateDemo, totalFrames2 } from './Video2.jsx'
import { KlewCombinedDemo, totalFramesCombined } from './Combined.jsx'
import { FPS } from './theme.js'

const Root = () => (
  <>
  <Composition
    id="KlewResourcesDemo"
    component={KlewResourcesDemo}
    durationInFrames={totalFrames}
    fps={FPS}
    width={1920}
    height={1080}
  />
  <Composition
    id="KlewInvestigateDemo"
    component={KlewInvestigateDemo}
    durationInFrames={totalFrames2}
    fps={FPS}
    width={1920}
    height={1080}
  />
  <Composition
    id="KlewDemo"
    component={KlewCombinedDemo}
    durationInFrames={totalFramesCombined}
    fps={FPS}
    width={1920}
    height={1080}
  />
  <Composition
    id="KlewDemoBare"
    component={KlewCombinedDemo}
    defaultProps={{ bare: true }}
    durationInFrames={totalFramesCombined}
    fps={FPS}
    width={1920}
    height={1080}
  />
  </>
)

registerRoot(Root)
