import * as React from 'react';
import { useHistory, useLocation } from 'react-router-dom';
import {
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  PageSection,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const BuildCreatePage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const initialProjectId = params.get('project_id') || '';

  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [projectId, setProjectId] = React.useState(initialProjectId);
  const [triggeredBy, setTriggeredBy] = React.useState('');

  const handleTrigger = async () => {
    if (!projectId) {
      setError('Project ID is required');
      return;
    }
    setSubmitting(true);
    setError(null);

    try {
      const api = createAPIClient();
      const created = await api.builds.create({
        project_id: projectId,
        status: 'pending',
        ...(triggeredBy ? { triggered_by: triggeredBy } : {}),
      });
      history.push(`/trex-console/builds/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/builds')}
            component="button"
          >
            Builds
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Trigger Build</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Trigger Build</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form style={{ maxWidth: 600 }}>
          <FormGroup label="Project ID" fieldId="field-project_id" isRequired>
            <TextInput
              id="field-project_id"
              value={projectId}
              onChange={(_e, val) => setProjectId(val)}
              placeholder="Enter Project ID"
              isRequired
              isDisabled={!!initialProjectId}
            />
          </FormGroup>
          <FormGroup label="Triggered By" fieldId="field-triggered_by">
            <TextInput
              id="field-triggered_by"
              value={triggeredBy}
              onChange={(_e, val) => setTriggeredBy(val)}
              placeholder="your-email@example.com"
            />
          </FormGroup>
          <Button
            variant="primary"
            onClick={handleTrigger}
            isLoading={submitting}
            isDisabled={submitting || !projectId}
          >
            Trigger Build
          </Button>
          <Button
            variant="link"
            onClick={() => history.push('/trex-console/builds')}
            style={{ marginLeft: 8 }}
          >
            Cancel
          </Button>
        </Form>
      </PageSection>
    </>
  );
};

export default BuildCreatePage;
